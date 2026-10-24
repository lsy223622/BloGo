const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');

function browser(reply){
 let main,requests=0;
 const context={document:{documentElement:{dataset:{}},readyState:'loading',querySelector:s=>s==='#main'?main:null,querySelectorAll:()=>[],addEventListener(){}},window:{matchMedia:()=>({matches:false})},localStorage:{getItem(){}},clearTimeout(){},setTimeout(){},fetch:async()=>{requests++;if(reply instanceof Error)throw reply;return {ok:true,json:async()=>reply}}};
 vm.runInNewContext(fs.readFileSync('static/assets/theme.js','utf8'),context);
 return {get requests(){return requests},async show(ids,enabled=true){const nodes=ids.map(id=>({dataset:{viewsFor:id},hidden:true,textContent:''}));main={dataset:{statsEnabled:String(enabled)},querySelectorAll:s=>s==='[data-views-for]'?nodes:[]};await context.window.BloGoPage.init();await new Promise(resolve=>setImmediate(resolve));return nodes},async repeat(){await context.window.BloGoPage.init()}};
}
test('one public JSON fills all cumulative counts and Swup lists without duplicate initialization',async()=>{
 const b=browser({pages:{a:{views:0},b:{views:1234}},popular_30d:[]});
 const nodes=await b.show(['a','b','missing']);assert.equal(b.requests,1);assert.equal(nodes[0].hidden,false);assert.match(nodes[0].textContent,/^0 /);assert.match(nodes[1].textContent,/1,234/);assert.equal(nodes[2].hidden,true);
 await b.repeat();assert.equal(b.requests,1);const next=await b.show(['b','a']);assert.equal(b.requests,2);assert.equal(next[0].hidden,false);
});
test('disabled statistics never fetch and unavailable or invalid counts stay hidden',async()=>{
 const disabled=browser({pages:{a:{views:10}}});assert.equal((await disabled.show(['a'],false))[0].hidden,true);assert.equal(disabled.requests,0);
 for(const reply of [Error('offline'),{pages:{a:{views:-1}}},{pages:{a:{views:'12'}}}]){const b=browser(reply);assert.equal((await b.show(['a']))[0].hidden,true)}
});
