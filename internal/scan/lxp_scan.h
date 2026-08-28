/* C ABI for liblxp_scan (see ../../../lxp-scan/src/ffi.rs). */
#ifndef LXP_SCAN_H
#define LXP_SCAN_H

/* Run a scan tool. root: workspace path. request_json: {"name":...,"arguments":{...}}.
 * Returns a heap JSON string {"content":[{"type":"text","text":...}],"isError":bool}.
 * Never null. Free with lxp_scan_free. */
char *lxp_scan_call(const char *root, const char *request_json);

/* Free a string returned by lxp_scan_call. Null is a no-op. */
void lxp_scan_free(char *ptr);

#endif /* LXP_SCAN_H */
