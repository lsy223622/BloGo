const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');

function element(tag,attrs={},text=''){
 return {tag,attrs,text,cloneNode(){return element(tag,{...attrs},text)},getAttribute(key){return this.attrs[key]??null},remove(){this.parent.children.splice(this.parent.children.indexOf(this),1)}};
}
function documentFor(title,url,schema){
 const head={children:[],append(node){node.parent=this;this.children.push(node)},querySelectorAll(selector){return this.children.filter(node=>node.tag==='meta'||node.tag==='link'&&node.attrs.rel==='canonical'||node.tag==='script'&&node.attrs['data-blogo-schema']!==undefined&&selector.includes('data-blogo-schema'))}};
 head.append(element('meta',{property:'og:title',content:title}));head.append(element('link',{rel:'canonical',href:url}));if(schema)head.append(element('script',{type:'application/ld+json','data-blogo-schema':''},JSON.stringify(schema)));
 const main={dataset:{pageId:title,siteTimezone:'Asia/Shanghai'},querySelector:()=>null};
 return {head,main,title,body:{dataset:{}},documentElement:{getAttribute:()=>null,removeAttribute(){}},querySelector:s=>s==='#main'?main:null,querySelectorAll:()=>[],dispatchEvent(){}};
}
test('Swup head updates replace generated article JSON-LD and remove it on lists',async()=>{
 const current=documentFor('列表','https://example.test/');const hooks={};
 class Swup{constructor(){this.hooks={on(){},before(){},replace(name,fn){hooks[name]=fn}}}}
 const window={Swup,SwupScrollPlugin:class{},BloGoPage:{destroy(){},async init(){}},fetch(){},DOMParser:class{},history:{pushState(){},replaceState(){}},matchMedia:()=>({matches:false})};
 vm.runInNewContext(fs.readFileSync('static/assets/navigation.js','utf8'),{document:current,window,location:{href:'https://example.test/',origin:'https://example.test'},URL,Event:class{}});
 for(const incoming of [documentFor('文章A','https://example.test/posts/a/',{headline:'A'}),documentFor('文章B','https://example.test/posts/b/',{headline:'B'}),documentFor('标签 · 第 2 页','https://example.test/tags/a/page/2/')]){
  await hooks['content:replace']({to:{document:incoming}},{},async()=>{current.main=incoming.main;current.title=incoming.title});
  const schemas=current.head.children.filter(node=>node.tag==='script');assert.equal(schemas.length,incoming.head.children.filter(node=>node.tag==='script').length);if(schemas.length)assert.equal(schemas[0].text,incoming.head.children.find(node=>node.tag==='script').text);
  assert.equal(current.head.children.find(node=>node.tag==='meta').attrs.content,incoming.title);assert.equal(current.head.children.find(node=>node.tag==='link').attrs.href,incoming.head.children.find(node=>node.tag==='link').attrs.href);
 }
});
