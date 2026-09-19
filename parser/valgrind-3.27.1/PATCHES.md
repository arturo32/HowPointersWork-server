# Valgrind patches

This vendored source is not stock Valgrind. It carries a hand-written patch that makes
`memcheck` emit a line-by-line execution trace (variable names, types, values, stack/heap
state) — this is the mechanism the whole "How Pointers Work" visualizer is built on.

## Origin

- Original patch by Philip Guo (`pgbovine`), written for his Online Python Tutor / OPT
  project, originally against Valgrind 3.11.0 (2015).
- Adds two memcheck-only CLI options: `--source-filename=<path>` and `--trace-filename=<path>`.
- At every instruction boundary in the traced file, it walks the DWARF type info Valgrind
  already has in memory and JSON-dumps live variable values (using memcheck's own
  uninitialized/unaddressable tracking) to the trace file, along with mirrored `ERROR: ...`
  lines for any memcheck errors.

## Scope

7 files touched, ~1,330 hand-written lines (plus a ~1,500-line vendored JSON encoder inlined
for portability, since Valgrind tools can't link a system JSON library):

- `memcheck/mc_main.c` — CLI options, setup/teardown
- `memcheck/mc_include.h` — shared declarations
- `memcheck/mc_translate.c` — per-instruction instrumentation hook (`pg_trace_inst`)
- `memcheck/mc_errors.c` — mirrors each memcheck error into the trace
- `coregrind/m_debuginfo/tytypes.c` + `priv_tytypes.h` — DWARF type-directed value
  printing (`pg_pp_varinfo`)
- `coregrind/m_debuginfo/debuginfo.c` + `include/pub_tool_debuginfo.h` — global/local
  variable traversal + the inlined JSON lib

## Port history

- 2024–2026 (`arturo32`): 3 small toolchain-compat fixes to keep the 3.11.0-era patch
  compiling under gcc ≥10 — a `-fno-common` tentative-definition fix, a cross-module
  function-pointer return-type fix, and a `-Wformat` fix.
- 2026-09: ported forward onto Valgrind 3.27.1 to support a modern Debian 12 / gcc 12
  toolchain (the old 3.11.0 build couldn't parse debug info from glibc ≥2.31). The patch
  was extracted as a clean diff against pristine 3.11.0 and replayed onto pristine 3.27.1;
  32 of 34 hunks applied automatically. The rest needed manual fixes:
  - Several Valgrind APIs (`VG_(describe_addr)`, `VG_(get_filename)`, `VG_(get_fnname)`,
    `VG_(get_fnname_kind_from_IP)`, `VG_(get_linenum)`) gained a `DiEpoch` parameter since
    3.11.0 — call sites updated to pass `VG_(current_DiEpoch)()`.
  - Two hunks had landed in the wrong place due to patch fuzz-matching (custom CLI options
    spliced into the middle of an unrelated option handler; a cleanup call landed inside a
    stats-printing function's argument list) — relocated to their correct spots.
  - Confirmed PIE executables trace correctly with no extra compiler flags needed.
  - Validated end-to-end: a simple pointer program and a deliberate use-after-free both
    traced correctly through to the final JSON format the Go server/frontend expect.
- 2026-09 (follow-up, found via a real C++ submission): stock Valgrind's
  `ML_(evaluate_Dwarf3_Expr)` (in `coregrind/m_debuginfo/d3basics.c`, not part of the
  pgbovine patch) unconditionally logs `warning: evaluate_Dwarf3_Expr: unhandled DW_OP_
  0x..` for every DWARF opcode it doesn't implement — including `DW_OP_form_tls_address`,
  which libstdc++/`<iostream>`'s thread-local internals use constantly. Because our patch
  calls into this evaluator on every traced instruction (not just on error, like stock
  memcheck does), a 15-line C++ program using `<iostream>` produced ~1,900 of these lines.
  That, combined with a pre-existing bug in `parser/wsgi_backend.py`'s `generate_trace()`
  (it passed the entire combined valgrind stdout+stderr blob to a downstream `Popen` as a
  single command-line argument, instead of the short end-of-trace error message), blew past
  the OS's `ARG_MAX` and crashed with `OSError: [Errno 7] Argument list too long`. Fixed by
  removing the noisy log line (the graceful failure/fallback behavior is unchanged) and by
  passing the correct short value through in `wsgi_backend.py`.
