# lssrv Changelog

Global changelog file. Please use markdown formatting and write most recent changes on top. Dates use ISO formatting.

## 20240122
- proj: Release version 2.0.0
- docs: Update documentation and files.
- proj: Update configuration file with the new example.
- feat: Added ability to customize table title.
- feat: Added ability to filter out partitions from final table.
- feat: Added ability to set queue state file location.
- feat: Added ability to auto-find config file with fallbacks.
- feat: Added ability to run without partial and no config (bulletproof defaults).
- feat: Add `flag` support to add versioning information.
- feat: `lssrv` can show its own version now.

## 20240121
- refactor: Remove Python codebase.
- refactor: Update `cron.d/lssrv_helper` to be compatible with the Go version.
- refactor: Add code comments.
- feat: Carry information about queue state file, so we can show last update date. 
- proj: Initial version is done.
- refactor: Replace BubbleTea with go-pretty's table module.
- refactor: Organize code to improve readability.
- refactor: Fix some debug messages' terminology.
- refactor: Change state file location for production.

## 20240120
- feat: Start to integrate `bubbletea` TUI library.
- feat: Implemented a simple table view.
- feat: Made the program exit immediately after running.
- Refactor: Fine tune table colors.

## 20240116
- refactor: Some small changes before implementing table view.
- refactor: A complete refactoring run for variable and function names.

## 20240115
- feat: Implement squeue file parsing logic.
- feat: Implement first version of presentation logic.

## 20240114
- refactor: Change most of the `int` fields to `string` since we won't be processing them as numbers.
- doc: Remove blank lines after dates since Markdown parses them fine.
- feat: Complete parsing and calculating partition properties.
- feat: Start implementing queue state parsing and job counting.

## 20240112
- feat: Start parsing the command output into the `Partition` structures.
- feat: Implemented `sinfo` integration.
- feat: Implemented header verification to check output compatibility.
- refactor: Move go module files to correct places in the tree.
- refactor: Rename `lssrv` to `lssrv-launcher.py` to prevent name clashes during `go build`.

## 20240111
- refactor: Start rewriting in Go.
- proj: Add data structures required for partitions.
- proj: Add Uber's Zap logging package for logging.
- proj: Enable Go modules.

## 20220606
- fix: Correct handling of multi-partition spanning jobs.
- fix: Increase partition field to 64 characters in cron job to make multi-partition jobs fit into the line.
- fluff: Improve f-strings based logging to prevent unnecessary type casts.
- fluff: Bump version to 0.0.5a20220606.
- fluff: Move code constants to the top for easier maintentance.

## 20220509
- Change all logging lines to f-strings to increase readability.
- Bump version to 0.0.5a20220509

## 20220506
- Added Eclipse PyDev project files to the repository.
- Add an argument parser to the code to prepare for future functionality, and help info.
- Change all variables to snake_case.
  - The properties coming from slurm are not changed. This is not a mistake.
- Added `# -*- coding: utf-8 -*-` line to the python file.

## 20220428
- Update `.gitignore` file.
- Fix cron file for `cron.d` folder installation.
- Add English `README.md` section.
- Fix some typos in the `README.md` file.
- First public release and initial upload.
