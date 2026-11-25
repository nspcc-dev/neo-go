# Interop pricing models

This directory contains interop measurements and pricing models.

## Content and format

- `benchmark_results/<full interop name>.csv` contains measurements for one
  interop, e.g. `System.Runtime.Notify.csv`.
- `models.ipynb` fits a model for every CSV, using all measured dependencies
  or just a constant for static interops.
- `requirements.txt` reuses the pinned opcode-model dependencies. The notebook
  loads the shared fitting helpers from `pkg/vm/opcodefee/src/regression.py`.

Each CSV has a header row. Its last column, `nops`, is the time of one operation
expressed in NOP execution times. Other columns describe the measured parameters:

| Column                                                           | Meaning                                         |
| ---------------------------------------------------------------- | ----------------------------------------------- |
| `refsdelta`                                                      | Change in the VM reference counter.             |
| `namelen`, `scriptlen`, `numbytes`                               | Method name, script, or returned byte length.   |
| `argscount`, `paramscount`                                       | Call arguments or event parameters.             |
| `permissionscount`, `methodscount`, `eventscount`, `groupscount` | Corresponding manifest entries.                 |
| `rulescount`                                                     | Evaluated witness rules.                        |
| `keyscount`, `pubkeyscount`                                      | Public keys passed to cryptographic operations. |
| `notificationscount`                                             | Notifications examined by `GetNotifications`.   |
| `itemscount`                                                     | Items in the notification payload.              |

Constant interop measurements have only `nops` and produce an intercept-only fit.

## Generating measurements

Set `generatePrices` in `pkg/core/interop/prices_test.go` to `true` and run from
the repository root:

```bash
go test ./pkg/core/interop -run '^TestGenerateInteropPriceCSV$' -timeout 0
```

Restore `generatePrices` to `false` afterwards. The test intentionally fails its
final guard while generation is enabled. It writes the full interop filenames
and column names documented above directly to `benchmark_results/`.

## Fitting models

With Python 3.12 or later installed:

```bash
cd pkg/core/interop/functionfee
python3 -m venv venv
source venv/bin/activate
python3 -m pip install -r requirements.txt
jupyter nbconvert --to markdown --execute --no-input --stdout models.ipynb
```

The notebook prints coefficients multiplied by `vm.OpcodePriceMultiplier` (1000).
