# Dynamic opcode pricing models

This directory contains the measurements and the scripts used to build the dynamic (parameter-dependent) opcode price models from `pkg/core/fee/opcode.go`.

## Files

- `benchmark_results/opcodes/<OPCODE>.csv` — execution time of a single opcode measured for a set of parameter values, one file per opcode (or per group of opcodes priced the same way, e.g. `ST.csv` for `ST*` opcodes).
- `models.ipynb` — Jupyter notebook that fits a model for every CSV file and prints the resulting weights.
- `src/regression.py` — fitting helpers used by the notebook:
  - `fit_weights` — linear least squares fit where samples are weighted inversely to their normalized parameter values, so that small parameter values (the most common case in real contracts) are fitted more precisely.

## CSV columns

Every row is a single measurement. The last column is the measured value; the other columns are the parameters it was measured with.

| Column         | Meaning                                                                                                                                                                         |
|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `type`         | Type of the stack item that in most cases serves as the operand of the opcode.                                                                                                  |
| `length`       | The number of elements in the compound type, the length of Buffer or ByteString, or the number of VM slot cells or stack elements involved in opcode handling.                  |
| `refsdelta`    | The total change of the VM reference counter performed by the opcode.                                                                                                          |
| `ncloneditems` | The number of items cloned by the opcode.                                                                                                                                       |
| `NOPs`         | Execution time of a single opcode expressed in `NOP` execution times (i.e. `1` is the time of one `NOP`).                                                                       |

Not every parameter is used for every `type`; the notebook drops the unused columns for such subsets (see `drop_columns` arguments).

## Reproducing the results

### Measurements

Every value was obtained by executing a script in the VM CLI:

1. For every data point, compose a script that executes the opcode with known parameters (`refsdelta`, `length`, etc.) many times for better precision: the script prepares the operands, executes the opcode and jumps back.
2. Execute the script with a gas limit in the VM CLI:
   ```
   NEO-GO-VM > loadbase64 --gas <limit> <base64 script>
   NEO-GO-VM > run
   ```
   and measure the execution time.
3. Measure the execution time of the loop's auxiliary instructions (everything except the opcode itself) the same way and subtract it; divide the result by the number of executed loop iterations to get the time of a single opcode.
4. Measure the `NOP` execution time the same way and divide the single opcode time by it to get the value in `NOPs`.

### Models

Install the dependencies and run the notebook from this directory. For every opcode, the notebook prints the fitted weights multiplied by `vm.OpcodePriceMultiplier`. The weights are listed in the order of the CSV parameter columns, and the last one is the intercept. They are used as is in `pkg/core/fee/opcode.go`, except that `NEWARRAY` and `NEWARRAYT` models must have the same intercept.
