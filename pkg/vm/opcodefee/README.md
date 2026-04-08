# Dynamic opcode pricing models

This directory contains the measurements and the scripts used to build the
dynamic (parameter-dependent) opcode price models from `pkg/core/fee/opcode.go`.

## Content

- `benchmark_results/<OPCODE>.csv` — execution time of a single opcode measured
  for a set of parameter values, one file per opcode (or per group of opcodes
  priced the same way, e.g. `ST.csv` for `ST*` opcodes). Refer to the
  [Benchmark result format](#benchmark-result-format) section for details
  about file format.
- `models.ipynb` — Jupyter notebook that fits a model for every CSV file and
  prints the resulting weights.
- `src/regression.py` — fitting helpers used by the notebook:
  - `fit_weights` — linear least squares fit where samples are weighted
    inversely to their normalized parameter values, so that small parameter
    values (the most common case in real contracts) are fitted more precisely.
  - `print_weights` — prints the fitted weights labeled with the corresponding
    parameter names.

### Benchmark result format

Benchmark results are stored in CSV files located under `benchmark_results`.
Every file contains measurements of a single opcode (or of a group of opcodes
priced the same way) with a header row naming the columns. Every row is a
single measurement: the last column is the measured value, the other columns
are the parameters it was measured with.

| Column | Meaning |
| --- | --- |
| `type` | Type of the stack item that in most cases serves as the operand of the opcode. |
| `length` | The number of elements in the compound type, or the length of Buffer or ByteString, or the number of VM slot cells or stack elements involved in opcode handling. |
| `refsdelta` | The total change of the VM reference counter caused by the opcode processing. |
| `ncloneditems` | The number of items cloned during the opcode execution. |
| `nops` | Execution time of a single opcode expressed in `NOP` execution times (i.e. `1` represents the execution time of one `NOP`). |

Not every parameter is used for every `type`; the notebook drops the unused
columns for such subsets (see `drop_columns` arguments).

## Benchmarks

### Evaluation

Every value was obtained by executing a script in the VM CLI:

1. Instrument the VM to accumulate the execution time and the number of
   executions of every opcode (e.g. measure the time of every `VM.execute`
   call in `VM.step` and accumulate it per opcode).
2. For every data point, compose a script that executes the opcode with known
   parameters (`refsdelta`, `length`, etc.) in a loop: the script prepares the
   operands, executes the opcode and jumps back.
3. Execute the script with a gas limit in the VM CLI:
   ```
   NEO-GO-VM > loadbase64 --gas <limit> <base64 script>
   NEO-GO-VM > run
   ```
   The loop runs until the gas is exhausted, so the higher the limit, the more
   precise the result. Divide the accumulated execution time of the opcode by
   the number of its executions to get the time of a single opcode.
4. Measure the `NOP` execution time the same way and divide the single opcode
   time by it to get the value in `nops`.

For example, the `256,26.458070` row of `DROP.csv` is obtained with the
`PUSHINT16 255; NEWARRAY; DROP; JMP -5` script (`Af8Aw0Ui+w==` in base64).
Every iteration creates an array of 255 elements and drops it, which changes
the reference counter by 256 (the array itself and its elements). The average
`DROP` execution time divided by the average `NOP` execution time gives
`26.458070`.

### Models

To build the models:

1. Make sure you have Python 3 installed.
2. Go to this directory:
   ```
   $ cd pkg/vm/opcodefee
   ```
3. Create and activate a virtual environment:
   ```
   $ python3 -m venv venv
   $ source venv/bin/activate
   ```
4. Install the dependencies:
   ```
   $ pip install numpy pandas nbconvert ipykernel
   ```
5. Run the notebook from the console:
   ```
   $ jupyter nbconvert --to markdown --execute --no-input --stdout models.ipynb
   ```

For every opcode, the notebook prints the fitted weights multiplied by
`vm.OpcodePriceMultiplier`. Every weight is labeled with the parameter it
corresponds to, `intercept` is the constant term, e.g.:
```
APPEND: refsdelta=97, ncloneditems=192, intercept=2714
```
The weights are used as is in `pkg/core/fee/opcode.go`, except that `NEWARRAY`
and `NEWARRAYT` models must have the same intercept.
