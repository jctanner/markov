{% raw %}
# Reproduce Jev integration tests with CPU Kev

This guide recreates the local Kev-0.8B setup used on 2026-09-27, then runs Markov's Jev example through authenticated inference and offline resume. It is self-contained within this repository; the original temporary directories are not required.

See the [recorded results](../notes/jev-kev-validation.md), [raw responses](../notes/jev-kev-validation.json), and [Jev language reference](../reference/jev.md).

## Tested environment and prerequisites

- Linux x86-64; AMD Ryzen 7 2700X (8 cores/16 threads, AVX2), 64 GB RAM.
- CPU-only PyTorch 2.8.0+cpu, FP32, eight OpenMP/MKL threads; no GPU or llama.cpp.
- Python 3.13.13, Transformers 5.17.0, PEFT 0.21.0, TypeSafe SDK 0.7.2.
- Git, Bash, curl, `uv`, Go compatible with Markov's `go.mod`, and Make installed.
- Internet access for GitHub, Python packages, and Hugging Face during initial setup.

The 0.8B server measured approximately 5.3 GiB resident RAM (5.4 GiB peak) on short workloads. Allow additional RAM for the OS and longer inputs. Its downloaded model cache occupied roughly 1.8 GB; Python dependencies and the source checkout need additional disk space. These are observed footprints, not minimum requirements.

Use disk-backed storage for the setup. The original test used `/tmp`, but `/tmp` is RAM-backed on that host and is not a durable cache. This guide uses a directory under the user's home instead. Adjust thread count on smaller machines.

## 1. Create the environment and pin Kev

Run from the **Markov repository root**. These commands assume a fresh setup directory; for an existing installation, verify its revision instead of cloning again.

```bash
export MARKOV_DIR="$PWD"
export KEV_TEST_DIR="$HOME/.local/share/markov-kev-test"
export KEV_SOURCE="$KEV_TEST_DIR/kev"
export KEV_PYTHON="$KEV_TEST_DIR/venv/bin/python"
export HF_HOME="$KEV_TEST_DIR/huggingface"
mkdir -p "$KEV_TEST_DIR"

uv python install 3.13.13
uv venv --python 3.13.13 "$KEV_TEST_DIR/venv"

git clone https://github.com/jaredpalmer/kev.git "$KEV_SOURCE"
git -C "$KEV_SOURCE" checkout --detach 5920c5fe4ca8e0970ed4209ac2c9b8e18bea5109
```

This Kev revision requires Python `>=3.12,<3.14`; do not use the host's Python 3.14. No Kev source patches were needed.

Install CPU Torch first from its dedicated index. Then install the recorded package versions and the pinned Kev checkout:

```bash
uv pip install --python "$KEV_PYTHON" \
  --index-url https://download.pytorch.org/whl/cpu \
  'torch==2.8.0+cpu'

uv pip install --python "$KEV_PYTHON" \
  --requirement "$MARKOV_DIR/docs/guides/kev-cpu-requirements.txt" \
  --editable "$KEV_SOURCE[serve]"

uv pip check --python "$KEV_PYTHON"
"$KEV_PYTHON" - <<'PY'
import sys, torch
from importlib.metadata import version
print(sys.version)
for package in ['torch', 'transformers', 'peft', 'typesafe-sdk']:
    print(package, version(package))
print('CPU capability:', torch.backends.cpu.get_cpu_capability())
print('CUDA available:', torch.cuda.is_available())
assert torch.__version__ == '2.8.0+cpu'
assert not torch.cuda.is_available()
PY
```

The [requirements snapshot](kev-cpu-requirements.txt) includes the original transitive dependency versions and test utilities, with the original machine-specific editable path removed. It is a version record, not a hash-locked wheel archive. The fresh-install recipe above is reconstructed from the successful environment; it has not been independently reinstalled into a second clean environment. Preserve source, package artifacts, and model cache if long-term availability is required.

## 2. Generate a local API key

Keep shell tracing (`set -x`) disabled. Generate a key once and save it outside the checkout:

```bash
"$KEV_PYTHON" - <<'PY'
import os, secrets
from pathlib import Path
path = Path(os.environ['KEV_TEST_DIR']) / 'api-key'
if not path.exists():
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(secrets.token_urlsafe(32))
else:
    path.chmod(0o600)
PY
export KEV_API_KEY="$(cat "$KEV_TEST_DIR/api-key")"
```

Both Kev and the Markov runner must receive this same value. The example's `api_key_env: KEV_API_KEY` is a credential reference, not the key itself. This local key is unrelated to a hosted TypeSafe API key.

## 3. Download and serve the pinned model

In this first terminal, start Kev in the foreground:

```bash
cd "$KEV_SOURCE"
OMP_NUM_THREADS=8 MKL_NUM_THREADS=8 \
KEV_BACKEND=torch KEV_DTYPE=fp32 \
KEV_FUSED=0 KEV_CUDA_GRAPHS=0 \
"$KEV_PYTHON" -u -m kev.serve \
  --run jaredpalmer/kev-0.8b@9a45d25eb2ab761841196625383fa1dff0e56c1e \
  --host 127.0.0.1 --port 18009
```

`HF_HOME` and `KEV_API_KEY` were exported above and are inherited by the process. The first start downloads the adapter/head, tokenizer, and base-model weights into `HF_HOME`. Wait for the Uvicorn startup message and a line reporting `cpu via torch (float32)`.

The adapter/head revision above pins `Qwen/Qwen3.5-0.8B-Base` revision `dc7cdfe2ee4154fa7e30f5b51ca41bfa40174e68`. The HTTP model alias `kev-latest` in Markov selects this loaded checkpoint; it does not change the server's loaded weights.

Reference-PyTorch fallback messages about `causal_conv1d` and `flash-linear-attention` are expected on this setup. They are slower CPU paths, not failures. Do not enable CUDA kernels to resolve them.

After the first successful load, later starts can add `HF_HUB_OFFLINE=1` before the same command. Do not set it for an empty cache. Startup/download time is separate from inference timing.

## 4. Verify authentication and server metadata

Open a **second terminal**, change to the Markov repository root, and recreate these environment variables (shell exports do not carry between terminals):

```bash
export MARKOV_DIR="$PWD"
export KEV_TEST_DIR="$HOME/.local/share/markov-kev-test"
export KEV_PYTHON="$KEV_TEST_DIR/venv/bin/python"
export KEV_API_KEY="$(cat "$KEV_TEST_DIR/api-key")"

# Missing and incorrect keys should each print 401.
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18009/v1/models
curl -s -o /dev/null -w '%{http_code}\n' \
  -H 'Authorization: Bearer incorrect-test-key' \
  http://127.0.0.1:18009/v1/models

# Use the environment-held key without placing it in the command arguments.
"$KEV_PYTHON" - <<'PY'
import json, os, urllib.request
request = urllib.request.Request(
    'http://127.0.0.1:18009/v1/models',
    headers={'Authorization': 'Bearer ' + os.environ['KEV_API_KEY']},
)
with urllib.request.urlopen(request, timeout=10) as response:
    data = json.load(response)
print(json.dumps(data, indent=2))
model = data['models'][0]
assert model['device'] == 'cpu'
assert model['backend'] == 'torch'
assert model['dtype'] == 'float32'
assert model['cuda_graphs'] is None
PY
```

A 401 with the intended key means the server and client keys differ. A connection error means the server is not ready, is bound elsewhere, or the environment blocks localhost networking.

## 5. Run Markov, pause, and resume without Kev

From the same second terminal:

```bash
cd "$MARKOV_DIR"
make build
./bin/markov validate examples/jev.yaml

# Use a fresh database directory to avoid reusing an earlier test run.
export KEV_RUN_DIR="$(mktemp -d "$KEV_TEST_DIR/run.XXXXXX")"
./bin/markov run examples/jev.yaml \
  --state-store "$KEV_RUN_DIR/state.db" --verbose
```

The workflow deliberately pauses at `review`; pausing is expected, not a failed model call. It exercises:

1. A native three-question `jev` step registered as `triage`.
2. An inline Noul condition with a durable decision receipt.
3. A decision-backed gate fact and an explicit human-approval pause.

Find the run ID from the command output, or read it from the fresh database:

```bash
export KEV_RUN_ID="$("$KEV_PYTHON" - <<'PY'
import os, sqlite3
from pathlib import Path
with sqlite3.connect(Path(os.environ['KEV_RUN_DIR']) / 'state.db') as db:
    run_id, status = db.execute(
        'SELECT run_id, status FROM runs WHERE parent_run_id IS NULL'
    ).fetchone()
    assert status == 'paused', status
    print(run_id)
PY
)"
./bin/markov status "$KEV_RUN_ID" --steps --state-store "$KEV_RUN_DIR/state.db"
```

Now **press Ctrl+C in the first terminal** and wait for Kev to finish shutdown. Confirm it is stopped:

```bash
curl --connect-timeout 2 http://127.0.0.1:18009/v1/models
```

That request should fail to connect. Resume Markov in the second terminal:

```bash
./bin/markov resume "$KEV_RUN_ID" --var approved=true \
  --state-store "$KEV_RUN_DIR/state.db"
./bin/markov status "$KEV_RUN_ID" --steps --state-store "$KEV_RUN_DIR/state.db"
```

Expect a completed run and a completed `verify` step. Successful resume with no server proves the registered output and model-backed decisions were restored from persistence. Do not change the ticket, decision version, connection, or refresh token during this check. The `resume` command does not support `--verbose`.

For a repeat, restart Kev using the retained cache and choose a new `KEV_RUN_DIR`. The key and model weights can be reused. Retain the database as test evidence; no automatic cleanup is required.

## Optional: upstream API conformance suite

The original independent Kev test passed all 10 tests in `tests/test_api.py`, including the official TypeSafe SDK. That revision's tests make unauthenticated HTTP requests and use a hardcoded SDK placeholder key, so they will not pass against the authenticated server above without changes.

To repeat them unchanged, stop any server using port 18009 and start the same pinned model **without authentication**, still bound only to localhost:

```bash
# First terminal, with the setup variables from sections 1–3 available:
cd "$KEV_SOURCE"
env -u KEV_API_KEY HF_HOME="$KEV_TEST_DIR/huggingface" \
  OMP_NUM_THREADS=8 MKL_NUM_THREADS=8 \
  KEV_BACKEND=torch KEV_DTYPE=fp32 KEV_FUSED=0 KEV_CUDA_GRAPHS=0 \
  "$KEV_PYTHON" -u -m kev.serve \
  --run jaredpalmer/kev-0.8b@9a45d25eb2ab761841196625383fa1dff0e56c1e \
  --host 127.0.0.1 --port 18009
```

In the second terminal:

```bash
cd "$KEV_TEST_DIR/kev"
KEV_BASE_URL=http://127.0.0.1:18009 \
  "$KEV_PYTHON" -m pytest tests/test_api.py -q --tb=short
```

Stop this unauthenticated server when finished. Restart with `KEV_API_KEY` for the authenticated Markov test.

## Troubleshooting and limits

- CPU Torch must report `2.8.0+cpu`; a generic Torch installation may pull GPU packages or select a different device.
- Run only one server on port 18009. Model memory remains allocated until the server exits.
- If downloads or localhost sockets are blocked by a sandbox, permit those operations in the test environment; those errors do not establish model incompatibility.
- `HF_HUB_OFFLINE=1` requires all referenced artifacts in the same cache path used at first load.
- Markov may log unavailable Kubernetes configuration. This example uses no Kubernetes Jobs and needs no cluster.
- The tested Markov tree passed `make test`, `make build`, and `make vet`. See the validation report for the unrelated baseline formatting failures in `make lint`.
- These checks establish protocol, authentication, and replay behavior. They do not establish decision accuracy or concurrency performance.
{% endraw %}
