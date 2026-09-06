"""Export this checkout's public plugin SDK as an explicit local Go proxy.

No replace directive or sibling-source dependency is required by plugin repos.
This is a development artifact, not a published release.
"""
import argparse
import datetime
import hashlib
import json
from pathlib import Path
import subprocess
import zipfile

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parent.parent
module = 'github.com/yottaapp/yotta'
names = subprocess.check_output(['git', 'ls-files', '-co', '--exclude-standard'], cwd=root, text=True, encoding='utf-8').splitlines()
files = {p: (root / p).read_bytes() for p in ['go.mod', 'go.sum', 'LICENSE']}
for name in names:
    path = root / name
    if path.is_file() and name.startswith(('internal/', 'pkg/', 'sdk/', 'contracts/')):
        files[name] = path.read_bytes()
digest = hashlib.sha256()
for name, content in sorted(files.items()):
    digest.update(name.encode()); digest.update(b'\0'); digest.update(content)
version = 'v0.0.0-local.h' + digest.hexdigest()[:16]
proxy = Path(args.out).resolve()
directory = proxy / module / '@v'
directory.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(directory / (version + '.zip'), 'w', zipfile.ZIP_DEFLATED) as archive:
    for name, content in sorted(files.items()):
        archive.writestr(module + '@' + version + '/' + name, content)
(directory / (version + '.mod')).write_bytes(files['go.mod'])
(directory / (version + '.info')).write_text(json.dumps({'Version': version, 'Time': datetime.datetime.now(datetime.timezone.utc).isoformat()}), encoding='utf-8')
(directory / 'list').write_text(version + '\n', encoding='utf-8')
print(json.dumps({'version': version, 'proxy': proxy.as_uri()}, ensure_ascii=False))
