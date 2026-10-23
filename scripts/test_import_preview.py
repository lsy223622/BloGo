import importlib.util
import tempfile
import unittest
from pathlib import Path


class ImportPreviewTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location('import_preview', Path(__file__).with_name('import-preview.py'))
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)

    def test_toc_conversion_preserves_fenced_examples(self):
        body = '# 目录\n\n[TOC]\n\n# 正文\n\n```markdown\n[TOC]\n```\n'
        converted, count = self.module.convert_toc(body)
        self.assertEqual(converted, '\n# 正文\n\n```markdown\n[TOC]\n```\n')
        self.assertEqual(count, 1)

    def test_dates_do_not_mistake_release_tables_for_publication(self):
        self.assertEqual(self.module.publication_date('| v4 | 2026-02-05 | release |'), None)
        self.assertEqual(self.module.publication_date('- 2024-05-14 第一次发布'), '2024-05-14')

    def test_import_keeps_source_and_uses_only_confirmed_publication(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); source=root/'source'; source.mkdir(); older=root/'older'; older.mkdir()
            project=root/'project'; (project/'static').mkdir(parents=True)
            (project/'static'/'test.txt').write_text('asset',encoding='utf-8')
            (source/'文章.md').write_text('# 目录\n[TOC]\n\n# 内容\n- 2024-05-14 第一次发布\n',encoding='utf-8')
            (source/'未确认.md').write_text('| v1 | 2026-02-05 | release |',encoding='utf-8')
            (source/'关于.md').write_text('',encoding='utf-8')
            (source/'友链.md').write_text('- 名称：真实名称\n- 描述：真实描述',encoding='utf-8')
            before={p.name:p.read_bytes() for p in source.iterdir()}
            result=self.module.import_preview(source,older,None,project/'output'/'trial',project)
            self.assertEqual({p.name:p.read_bytes() for p in source.iterdir()},before)
            imported=[e for e in result['entries'] if e['action']=='imported']
            self.assertEqual(len(imported),3)
            by_name={e['source_name']:e for e in imported}
            self.assertEqual(by_name['文章.md']['status'],'public')
            self.assertEqual(by_name['未确认.md']['status'],'draft')
            self.assertIsNone(by_name['未确认.md']['published'])
            self.assertEqual(by_name['友链.md']['kind'],'page')
            self.assertIsNone(by_name['友链.md']['published'])
            self.assertEqual(result['site']['title'],'真实名称')
            again=self.module.import_preview(source,older,None,project/'output'/'trial',project)
            self.assertEqual([e.get('id') for e in result['entries']],[e.get('id') for e in again['entries']])


if __name__=='__main__': unittest.main()
