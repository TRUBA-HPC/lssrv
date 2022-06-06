# lssrv Changelog

Global changelog file. Please use markdown formatting and write most recent changes on top. Dates use ISO formatting.

## 20220606

- fix: Correct handling of multi-partition spanning jobs.
- fluff: Improve f-strings based logging to prevent unnecessary type casts.
- fluff: Bump version to 0.0.5a20220606.

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
