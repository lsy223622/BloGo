import importlib.util
import pathlib
import tarfile
import tempfile
import unittest
import zipfile

spec = importlib.util.spec_from_file_location("package_release", pathlib.Path(__file__).with_name("package-release.py"))
package = importlib.util.module_from_spec(spec)
spec.loader.exec_module(package)


class PackageTest(unittest.TestCase):
    def test_whitelist_and_executable_permissions(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            for file in ["config.yaml", "content/site.yaml", "content/posts/hello.md", "templates/page.html", "static/assets/theme.js", "static/vendor/library/LICENSE", "deploy/blogo.service", "docs/configuration.md", "README.md", "LICENSE", "data/statistics/visitor.key", "output/personal.md", ".git/config", "scripts/private.py"]:
                target = root / file
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(file, encoding="utf-8")
            binary = root / "compiled-blog"
            binary.write_bytes(b"binary")
            licenses = root / "licenses"
            licenses.mkdir()
            (licenses / "dependency.txt").write_text("license", encoding="utf-8")
            output = root / "packages"
            for platform in ["linux-amd64", "linux-arm64", "windows-amd64"]:
                archive = package.make_package(root, binary, platform, "v0.1.0-test", output, licenses)
                if archive.suffix == ".zip":
                    with zipfile.ZipFile(archive) as opened:
                        names = opened.namelist()
                else:
                    with tarfile.open(archive) as opened:
                        names = opened.getnames()
                        executable = next(x for x in opened if x.name.endswith("/blog"))
                        self.assertEqual(executable.mode & 0o777, 0o755)
                entries = {name.split("/", 1)[1] for name in names}
                self.assertIn("content/site.yaml", entries)
                self.assertIn("deploy/blogo.service", entries)
                self.assertIn("licenses/dependency.txt", entries)
                self.assertIn("blog.exe" if platform.startswith("windows") else "blog", entries)
                self.assertFalse(any(name.startswith(("data/", "output/", ".git/", "scripts/")) for name in entries))


if __name__ == "__main__":
    unittest.main()
