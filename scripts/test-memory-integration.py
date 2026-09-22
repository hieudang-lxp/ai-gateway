#!/usr/bin/env python3
"""Black-box two-repository test with fixture transcripts and real dependency probes.

Requires the Graphiti virtualenv, Go, running Ollama/Neo4j, and local port access.
Never reads real transcripts or runs extraction against the personal namespace.
"""
import argparse
import json
import os
from pathlib import Path
import socket
import sqlite3
import subprocess
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, build_opener, ProxyHandler


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait_for(check, label, seconds=45):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            value = check()
            if value:
                return value
        except (OSError, URLError, TimeoutError, sqlite3.OperationalError):
            pass
        time.sleep(0.2)
    raise AssertionError(f"timed out: {label}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--graphiti-root", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    graphiti = args.graphiti_root.resolve()
    python = graphiti / ".venv/bin/python"
    opener = build_opener(ProxyHandler({}))
    token = "isolated-integration-token"
    processes = []

    def request(base, path, body=None, authorized=True):
        headers = {"Content-Type": "application/json"}
        if authorized:
            headers["Authorization"] = "Bearer " + token
        req = Request(base + path, data=None if body is None else json.dumps(body).encode(), headers=headers)
        try:
            response = opener.open(req, timeout=12)
        except HTTPError as exc:
            response = exc
        with response:
            return response.status, json.load(response)

    with tempfile.TemporaryDirectory(prefix="memory-integration-") as directory:
        temp = Path(directory)
        api_url = f"http://127.0.0.1:{free_port()}"
        sync_url = f"http://127.0.0.1:{free_port()}"
        binary = temp / "memory-sync"
        subprocess.run(["go", "build", "-o", str(binary), "./services/memory-sync"], cwd=root / "backend", check=True)
        claude = temp / "claude"
        codex = temp / "codex/sessions"
        claude.mkdir()
        codex.mkdir(parents=True)
        (temp / "codex/archived_sessions").mkdir()
        timestamp = "2026-09-22T05:00:00Z"

        def claude_message(index):
            return {"type": "user", "uuid": f"human-{index}", "sessionId": "fixture-claude", "timestamp": timestamp,
                    "message": {"role": "user", "content": "ok" if index == 1 else "thanks"}}

        transcript = claude / "fixture.jsonl"
        transcript.write_text(json.dumps(claude_message(1)) + "\n")
        (codex / "fixture.jsonl").write_text("\n".join(map(json.dumps, [
            {"type": "session_meta", "payload": {"id": "fixture-codex", "source": "vscode"}},
            {"type": "response_item", "timestamp": timestamp, "payload": {"type": "message", "role": "user", "content": [{"type": "input_text", "text": "yes"}]}},
            {"type": "event_msg", "timestamp": timestamp, "payload": {"type": "agent_message", "message": "excluded"}},
        ])) + "\n")
        (codex / "subagent.jsonl").write_text("\n".join(map(json.dumps, [
            {"type": "session_meta", "payload": {"id": "fixture-subagent", "source": {"subagent": {"thread_spawn": {"parent_thread_id": "fixture-codex"}}}}},
            {"type": "event_msg", "timestamp": timestamp, "payload": {"type": "user_message", "message": "excluded model-generated task"}},
        ])) + "\n")
        cursor_path = temp / "cursor.db"
        with sqlite3.connect(cursor_path) as db:
            db.execute("CREATE TABLE cursorDiskKV(key TEXT PRIMARY KEY, value BLOB)")
            db.executemany("INSERT INTO cursorDiskKV VALUES (?,?)", [
                ("composerData:fixture-cursor", json.dumps({"composerId": "fixture-cursor"})),
                ("bubbleId:fixture-cursor:b1", json.dumps({"bubbleId": "b1", "type": 1, "text": "hi", "createdAt": timestamp})),
                ("bubbleId:fixture-cursor:b2", json.dumps({"bubbleId": "b2", "type": 2, "text": "excluded", "createdAt": timestamp})),
            ])
        env = dict(os.environ, GRAPHITI_API_TOKEN=token, GRAPHITI_URL=api_url,
                   GRAPHITI_GROUP_ID="integration-fixtures", DATABASE_URL="", NO_PROXY="*", no_proxy="*")
        graph_db = temp / "graphiti.sqlite3"
        sender_db = temp / "sender.sqlite3"
        api_args = [str(python), "-m", "memory.api", "--db", str(graph_db), "--host", "127.0.0.1",
                    "--port", api_url.rsplit(":", 1)[1], "--no-legacy-import"]
        sync_args = [str(binary), "-addr", sync_url.removeprefix("http://"), "-db", str(sender_db),
                     "-claude-projects", str(claude), "-codex-home", str(temp / "codex"),
                     "-cursor-state", str(cursor_path), "-interval", "1s"]

        with (temp / "process.log").open("w") as log:
            def launch(command, cwd):
                proc = subprocess.Popen(command, cwd=cwd, env=env, stdout=log, stderr=log)
                processes.append(proc)
                return proc

            def stop(proc):
                if proc.poll() is None:
                    proc.terminate()
                    try:
                        proc.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        proc.kill()
                        proc.wait()

            def counts():
                status, result = request(api_url, "/v1/status")
                assert status == 200
                return result["queue"]

            def delivered(number):
                status, result = request(sync_url, "/_memory")
                return status == 200 and result.get("delivered") == number and result.get("pending") == 0

            try:
                api = launch(api_args, graphiti)
                wait_for(lambda: request(api_url, "/healthz")[0] == 200, "Graphiti API startup")
                status, ready = request(api_url, "/v1/status")
                assert status == 200 and ready["ready"], f"real dependency probes failed: {ready['dependencies']}"
                assert request(api_url, "/v1/status", authorized=False)[0] == 401
                sender = launch(sync_args, root)
                wait_for(lambda: delivered(3), "three-source durable delivery")
                assert counts()["pending"] == 3
                with sqlite3.connect(graph_db) as db:
                    records = [json.loads(row[0]) for row in db.execute("SELECT payload FROM jobs")]
                assert {record["source"] for record in records} == {"claude_code", "codex", "cursor"}
                assert {record["text"] for record in records} == {"ok", "yes", "hi"}
                status, replay = request(api_url, "/v1/episodes", {"version": 1, "messages": records})
                assert status == 202 and replay == {"accepted": 0, "duplicates": 3}
                conflict = dict(records[0], text="changed")
                fresh = dict(records[0], uuid="atomic-test")
                assert request(api_url, "/v1/episodes", {"version": 1, "messages": [fresh, conflict]})[0] == 409
                assert counts()["pending"] == 3, "conflicting batch partially committed"
                assert request(api_url, "/v1/episodes", {"version": 1, "messages": [fresh, {}]})[0] == 400
                assert counts()["pending"] == 3, "invalid batch partially committed"
                stop(sender)
                stop(api)
                with transcript.open("a") as stream:
                    stream.write(json.dumps(claude_message(2)) + "\n")
                sender = launch(sync_args, root)
                wait_for(lambda: request(sync_url, "/_memory")[1].get("state") == "waiting", "dependency outage state")
                api = launch(api_args, graphiti)
                wait_for(lambda: delivered(4), "recovery after both processes restart")
                assert counts()["pending"] == 4, "restart duplicated or lost records"
                print("PASS: real readiness/auth; all three sources; short user messages; assistant exclusion; atomic validation; replay/conflict; outage recovery; two-process restart.")
            finally:
                for proc in reversed(processes):
                    stop(proc)


if __name__ == "__main__":
    main()
