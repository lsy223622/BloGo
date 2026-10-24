"""Package compiled targets and an explicit runtime-resource whitelist."""
import argparse
import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import tarfile
import zipfile

FILES = ("config.yaml", "README.md", "LICENSE")
DIRECTORIES = ("content", "templates", "static", "docs", "deploy")
TARGETS = ("linux-amd64", "linux-arm64", "windows-amd64")


def runtime_files(root):
    for name in FILES:
        yield root / name, name
    for directory in DIRECTORIES:
        for file in sorted((root / directory).rglob("*")):
            if file.is_symlink():
                raise ValueError(f"runtime symlink: {file}")
            if file.is_file():
                yield file, file.relative_to(root).as_posix()


def collect_licenses(root, output):
    raw = subprocess.check_output(["go", "list", "-mod=readonly", "-deps", "-json", "./cmd/blog"], cwd=root, text=True, encoding="utf-8")
    decoder, modules = json.JSONDecoder(), {}
    while raw.strip():
        raw = raw.lstrip()
        module, end = decoder.raw_decode(raw)
        dependency = module.get("Module")
        if dependency and not dependency.get("Main"):
            modules[dependency["Path"]] = dependency
        raw = raw[end:]
    output.mkdir(parents=True, exist_ok=True)
    records = []
    for module in sorted(modules.values(), key=lambda item: item["Path"]):
        if module.get("Main"):
            continue
        directory = pathlib.Path(module["Dir"])
        licenses = [file for file in directory.iterdir() if file.is_file() and file.name.upper().startswith(("LICENSE", "LICENCE", "COPYING", "NOTICE", "AUTHORS", "COPYRIGHT"))]
        if not licenses:
            raise ValueError(f"missing dependency license: {module['Path']}")
        destination = output / module["Path"]
        destination.mkdir(parents=True, exist_ok=True)
        for file in licenses:
            shutil.copyfile(file, destination / file.name)
        records.append({"module": module["Path"], "version": module["Version"]})
    (output / "modules.json").write_text(json.dumps(records, indent=2) + "\n", encoding="utf-8")
    goroot = pathlib.Path(subprocess.check_output(["go", "env", "GOROOT"], cwd=root, text=True).strip())
    shutil.copyfile(goroot / "LICENSE", output / "Go-LICENSE")


def make_package(root, binary, platform, version, output, licenses):
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?", version) or platform not in TARGETS:
        raise ValueError("expected semantic version and supported platform")
    windows = platform.startswith("windows")
    prefix = f"BloGo-{version}-{platform}" + ("-preview" if windows else "")
    output.mkdir(parents=True, exist_ok=True)
    archive = output / (prefix + (".zip" if windows else ".tar.gz"))
    entries = [(binary, "blog.exe" if windows else "blog"), *runtime_files(root)]
    entries += [(file, "licenses/" + file.relative_to(licenses).as_posix()) for file in sorted(licenses.rglob("*")) if file.is_file()]
    if windows:
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as writer:
            for file, name in entries:
                writer.write(file, prefix + "/" + name)
    else:
        with tarfile.open(archive, "w:gz", format=tarfile.PAX_FORMAT) as writer:
            for file, name in entries:
                info = writer.gettarinfo(str(file), prefix + "/" + name)
                info.uid = info.gid = 0
                info.uname = info.gname = ""
                info.mode = 0o755 if name == "blog" else 0o644
                with file.open("rb") as stream:
                    writer.addfile(info, stream)
    return archive


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--binary-dir", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parents[1]
    licenses = args.output / "go-licenses"
    collect_licenses(root, licenses)
    archives = []
    for target in TARGETS:
        binary = args.binary_dir / ("blog-" + target + (".exe" if target.startswith("windows") else ""))
        archives.append(make_package(root, binary, target, args.version, args.output, licenses))
    checksum = "".join(hashlib.sha256(file.read_bytes()).hexdigest() + "  " + file.name + "\n" for file in archives)
    (args.output / "SHA256SUMS").write_text(checksum, encoding="utf-8")
    for file in archives:
        print(file)


if __name__ == "__main__":
    main()
