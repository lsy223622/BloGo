"""Create local trial inputs from a Markdown working tree, without editing it."""
import argparse
import difflib
import hashlib
import json
import re
import shutil
import subprocess
import unicodedata
from pathlib import Path


def digest(data):
    return hashlib.sha256(data).hexdigest()


def convert_toc(body):
    output, fence, count = [], None, 0
    for line in body.splitlines(keepends=True):
        stripped = line.lstrip()
        marker = re.match(r'(`{3,}|~{3,})', stripped)
        if marker:
            value = marker.group(1)
            if fence is None:
                fence = (value[0], len(value))
            elif value[0] == fence[0] and len(value) >= fence[1]:
                fence = None
            output.append(line)
        elif fence is None and len(line)-len(stripped)<4 and stripped.strip().lower()=='[toc]':
            while output and not output[-1].strip(): output.pop()
            if output and re.fullmatch(r'#{1,6}\s+目录\s*',output[-1].strip()): output.pop()
            output.append(line)
            count += 1
        else:
            output.append(line)
    return ''.join(output), count


def publication_date(body):
    match=re.search(r'^\s*[-*]?\s*(\d{4}-\d{2}-\d{2})\s+(?:第一次发布|首次发布)',body,re.M)
    return match.group(1) if match else None


def comparison_key(name):
    name=unicodedata.normalize('NFKC',name).casefold()
    return re.sub(r'\(\d{4}-\d{2}-\d{2}\s*更新\)','',name).strip()


def first_git_date(source, name):
    # Trust is scoped to this user-selected repository for this read-only call.
    result=subprocess.run(['git','-c',f'safe.directory={source.as_posix()}','-C',str(source),
        'log','--follow','--diff-filter=A','--format=%aI','--',name],capture_output=True,text=True,encoding='utf-8')
    if result.returncode: return None
    dates=result.stdout.splitlines()
    return dates[-1] if dates else None


def import_preview(source, older, attachments, output, project):
    source,output,project=source.resolve(),output.resolve(),project.resolve()
    if not output.is_relative_to(project/'output'): raise ValueError('trial output must be inside project/output')
    if output.is_relative_to(source) or source.is_relative_to(output): raise ValueError('source and output must be separate')
    if not source.is_dir(): raise ValueError(f'source directory does not exist: {source}')
    originals={p.name:p.read_bytes() for p in sorted(source.glob('*.md'))}
    if not originals: raise ValueError(f'source contains no Markdown files: {source}')
    for data in originals.values(): data.decode('utf-8-sig')
    if older and not older.is_dir(): raise ValueError(f'older source directory does not exist: {older}')
    if attachments and not attachments.is_dir(): raise ValueError(f'attachment directory does not exist: {attachments}')
    marker=output/'import-map.json'
    if output.exists() and any(output.iterdir()) and not marker.exists(): raise ValueError('refusing to overwrite an unrecognized output directory')
    if marker.exists():
        previous=json.loads(marker.read_text(encoding='utf-8'))
        for entry in previous['entries']:
            if entry.get('output'):
                target=output/entry['output']
                if not target.resolve().is_relative_to(output/'content'): raise ValueError('unsafe previous mapping')
                target.unlink(missing_ok=True)
    output.mkdir(parents=True,exist_ok=True)
    (output/'content/posts').mkdir(parents=True,exist_ok=True)
    (output/'content/pages').mkdir(parents=True,exist_ok=True)
    shutil.copytree(project/'static',output/'static',dirs_exist_ok=True)
    entries=[]
    friends=originals.get('友链.md',b'').decode('utf-8-sig')
    title=re.search(r'^-\s*名称[：:]\s*(.+)$',friends,re.M)
    description=re.search(r'^-\s*描述[：:]\s*(.+)$',friends,re.M)
    site={'title':title.group(1).strip() if title else '本地博文预览',
          'description':description.group(1).strip() if description else ''}
    resources=[]
    for name,data in originals.items():
        body=data.decode('utf-8-sig')
        entry={'source':str(source/name),'source_name':name,'source_sha256':digest(data)}
        if not body.strip():
            entries.append({**entry,'action':'skipped','reason':'empty source; no biography invented'})
            continue
        page=name in ('关于.md','友链.md')
        identity={'关于.md':'about','友链.md':'links'}.get(name,'import-'+digest(name.encode())[:12])
        confirmed=publication_date(body)
        git_date=first_git_date(source,name)
        published=confirmed or (git_date if not page else None)
        basis='body explicitly records first publication' if confirmed else ('undated independent page' if page else ('Git first record; trial approximation, not actual publication' if git_date else 'unknown; no date invented'))
        status='public' if confirmed or name=='友链.md' else 'draft'
        converted,toc_count=convert_toc(body)
        changes=[]
        if toc_count: changes.append(f'preserved {toc_count} TOC marker(s) and removed adjacent empty TOC heading')
        for old,new in [('https://blog.lsy223622.com/category/tech/','/categories/tech/'),('https://blog.lsy223622.com/category/life/','/categories/life/')]:
            if old in converted:
                converted=converted.replace(old,new);changes.append(f'trial category link {old} -> {new}')
        for image in re.findall(r'!\[[^\]]*\]\(([^\s)]+)',converted):
            resources.append({'source_name':name,'url':image,'external':image.startswith(('https://','http://'))})
        life=any(word in name for word in ('雅思','西安','珍惜','日记'))
        category='life' if life else 'tech'
        tags=['notes']
        if life: tags.append('life')
        if 'Typecho' in name: tags.append('blog')
        metadata={'id':identity,'slug':identity,'title':Path(name).stem,'status':status,'stats':False}
        if page: metadata['permalink']='/'+identity+'/'
        if published: metadata['published']=published
        if not page: metadata.update(categories=[category],tags=tags)
        if confirmed:
            updates=re.findall(r'^\s*-\s*(\d{4}-\d{2}-\d{2})\s+(?:增加|更新|修改)',body,re.M)
            if updates: metadata['updated']=max([confirmed]+updates)
        relative=f'content/{"pages" if page else "posts"}/{identity}.md'
        # JSON scalars/arrays are YAML-compatible and need no extra dependency.
        front='---\n'+'\n'.join(f'{key}: {json.dumps(value,ensure_ascii=False)}' for key,value in metadata.items())+'\n---\n'
        (output/relative).write_text(front+converted,encoding='utf-8',newline='')
        entries.append({**entry,'action':'imported','kind':'page' if page else 'post','id':('page:' if page else 'post:')+identity,
            'url':'/'+identity+'/' if page else '/posts/'+identity+'/', 'output':relative,'status':status,'published':published,
            'date_basis':basis,'git_first_record':git_date,'body_publication':confirmed,'transformations':changes,
            'converted_body_sha256':digest(converted.encode()),'categories':[] if page else [category],'tags':[] if page else tags})
    comparisons=[]
    primary_by_key={comparison_key(name):name for name in originals}
    for path in sorted(older.glob('*.md')) if older else []:
        match=primary_by_key.get(comparison_key(path.name))
        old=path.read_bytes()
        if not match:
            comparisons.append({'older_source':str(path),'decision':'not imported; unmatched older source requires inspection','sha256':digest(old)})
        else:
            same=old==originals[match]
            delta=list(difflib.unified_diff(old.decode('utf-8-sig').splitlines(),originals[match].decode('utf-8-sig').splitlines())) if not same else []
            comparisons.append({'older_source':str(path),'primary':match,'decision':'identical; use primary' if same else 'different version; use current primary working tree','sha256':digest(old),'diff_lines':len(delta)})
    # Category/tag organization is solely for this local trial, not historical metadata.
    taxonomy={'schema_version':1,'categories':{'notes':{'name':'笔记'},'tech':{'name':'技术','parent':'notes'},'life':{'name':'生活','parent':'notes'}},
        'tags':{'notes':{'name':'随笔 / Notes'},'life':{'name':'生活'},'blog':{'name':'博客 · Typecho'}}}
    taxonomy.update(title=site['title'],subtitle=site['description'],page_size=5)
    (output/'content/site.yaml').write_text(json.dumps(taxonomy,ensure_ascii=False,indent=2),encoding='utf-8')
    template_path=Path(__import__('os').path.relpath(project/'templates',output)).as_posix()
    config={'server':{'base_url':'http://127.0.0.1:25646'},'timezone':'Asia/Shanghai',
        'paths':{'content':'content','templates':template_path,'static':'static'}}
    (output/'config.yaml').write_text(json.dumps(config,ensure_ascii=False,indent=2),encoding='utf-8')
    if any((source/name).read_bytes()!=data for name,data in originals.items()): raise RuntimeError('source changed during import; retry against a stable working tree')
    report={'schema_version':1,'source':str(source),'older_source':str(older) if older else None,'site':site,'entries':entries,
        'older_comparison':comparisons,'resources':resources,'attachment_candidates':[str(p) for p in sorted(attachments.glob('*')) if p.is_file()] if attachments else [],
        'notes':['Current working tree including uncommitted files is authoritative.','No mtime used for dates.','Only explicit body publication establishes Public posts; other posts are Draft for loopback preview.',
                 'Links is an undated independent page; empty About is skipped.','Categories/tags are trial organization inferred from titles.','External images retained; no remote media downloaded.']}
    marker.write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    rows=['# 本地试用导入说明','',f"站点：{site['title']} · {site['description']}",'',
        '来源是当前工作区（含未提交文件），原文件只读；没有使用文件 mtime。正文只转换旧 TOC 标记和两条已知分类链接，原始 SHA-256、转换记录与逐篇映射见 import-map.json。',
        '只有正文明确记录“第一次发布”的文章作为 Public；其余文章为 Draft，仅在回环预览显示。Git 首次记录日期仅为试用近似值，发行版本表格/图片日期不当作发布日期。',
        '分类标签为本地试用整理，非旧站原始元数据。Draft 不进入公共分类/归档；预览首页包含 Draft。空“关于”跳过，友链为无日期独立页面。',
        '外链图片保持原 URL；附件候选仅在映射中盘点，本批无本地图片引用则不复制。旧站 RSS/sitemap 核查在本轮超时，未据此猜测公开性。','',
        '预览：`go run ./cmd/blog dev --config output/personal-preview/config.yaml --output site --listen 127.0.0.1:8771`','',
        '| 源文件 | ID / URL | 状态 | 日期 | 依据 |','|---|---|---|---|---|']
    for e in entries:
        rows.append(f"| {e['source_name']} | {e.get('id','跳过')} {e.get('url','')} | {e.get('status',e.get('reason'))} | {e.get('published') or '无'} | {e.get('date_basis','空文件')} |")
    (output/'IMPORT.md').write_text('\n'.join(rows)+'\n',encoding='utf-8')
    return report


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,required=True)
    parser.add_argument('--older',type=Path)
    parser.add_argument('--attachments',type=Path)
    parser.add_argument('--output',type=Path,default=Path('output/personal-preview'))
    args=parser.parse_args()
    result=import_preview(args.source,args.older,args.attachments,args.output,Path(__file__).resolve().parents[1])
    print(json.dumps({'imported':sum(e['action']=='imported' for e in result['entries']),'skipped':sum(e['action']=='skipped' for e in result['entries']),
        'posts':sum(e.get('kind')=='post' for e in result['entries']),'pages':sum(e.get('kind')=='page' for e in result['entries']),
        'unmatched_older':sum('unmatched' in e['decision'] for e in result['older_comparison'])},ensure_ascii=False))


if __name__=='__main__': main()
