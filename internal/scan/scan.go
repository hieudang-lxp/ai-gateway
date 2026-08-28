// Package scan is the cgo bridge to liblxp_scan (the Rust analyzer).
package scan

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -L${SRCDIR}/../../lib -llxp_scan -Wl,-rpath,${SRCDIR}/../../lib
#include <stdlib.h>
#include "lxp_scan.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// Result is the parsed output of one tool call.
type Result struct {
	Text    string
	IsError bool
}

// Call invokes the lxp-scan tool `name` with `args` against `root`, in-process
// via cgo. When jsonMode is true the tool's canonical JSON is returned in
// Result.Text (for typed responses); otherwise Result.Text is the markdown/text
// pack. The returned C string is always freed.
func Call(root, name string, args map[string]any, jsonMode bool) (Result, error) {
	req := map[string]any{"name": name, "arguments": args}
	if jsonMode {
		req["format"] = "json"
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	cRoot := C.CString(root)
	defer C.free(unsafe.Pointer(cRoot))
	cReq := C.CString(string(reqJSON))
	defer C.free(unsafe.Pointer(cReq))

	out := C.lxp_scan_call(cRoot, cReq)
	if out == nil {
		return Result{}, fmt.Errorf("lxp_scan_call returned null")
	}
	defer C.lxp_scan_free(out)

	raw := C.GoString(out)
	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return Result{}, fmt.Errorf("decode result: %w (raw=%q)", err, raw)
	}
	r := Result{IsError: res.IsError}
	if len(res.Content) > 0 {
		r.Text = res.Content[0].Text
	}
	return r, nil
}
