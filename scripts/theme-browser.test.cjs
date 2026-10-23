const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

function browser({saved, dark=false, denied=false}={}) {
  const root={dataset:{}}, handlers={}, attributes={};
  const toggle={dataset:{},setAttribute:(key,value)=>attributes[key]=value,addEventListener:(key,fn)=>handlers[key]=fn};
  const media={matches:dark,addEventListener:(_,fn)=>media.change=fn};
  const storage=new Map(saved ? [['blogo-theme',saved]] : []);
  const context={
    document:{documentElement:root,querySelector:selector=>selector==='.theme-toggle'?toggle:null,querySelectorAll:()=>[],addEventListener:()=>{}},
    window:{matchMedia:()=>media},
    localStorage:{getItem:key=>{if(denied) throw Error('denied');return storage.get(key);},setItem:(key,value)=>{if(denied) throw Error('denied');storage.set(key,value);}},
    fetch:()=>Promise.reject(Error('no stats')),setTimeout
  };
  vm.runInNewContext(fs.readFileSync('static/assets/theme.js','utf8'),context);
  return {root,toggle,attributes,storage,click:()=>handlers.click(),system(value){media.matches=value;media.change();}};
}

test('theme cycles system, light, dark and persists the selected mode',()=>{
  const b=browser();
  assert.equal(b.toggle.dataset.mode,'system');
  assert.equal(b.toggle.dataset.tooltip,'系统');
  for(const [mode,name] of [['light','浅色'],['dark','深色'],['system','系统']]) {
    b.click();
    assert.equal(b.toggle.dataset.mode,mode);
    assert.equal(b.toggle.dataset.tooltip,name);
    assert.equal(b.storage.get('blogo-theme'),mode);
    assert.match(b.attributes['aria-label'],new RegExp(name));
  }
  assert.equal(browser({saved:'dark'}).root.dataset.theme,'dark');
});

test('only system mode follows operating system color changes',()=>{
  const b=browser({dark:true});
  assert.equal(b.root.dataset.theme,'dark');
  b.system(false);
  assert.equal(b.root.dataset.theme,'light');
  b.click();
  b.system(true);
  assert.equal(b.root.dataset.theme,'light');
  b.click();
  b.system(false);
  assert.equal(b.root.dataset.theme,'dark');
  b.click();
  assert.equal(b.root.dataset.theme,'light');
});

test('unavailable storage and invalid saved settings keep the control usable',()=>{
  for(const options of [{denied:true},{saved:'invalid'}]) {
    const b=browser(options);
    assert.equal(b.toggle.dataset.mode,'system');
    b.click();
    assert.equal(b.root.dataset.theme,'light');
    b.click();
    assert.equal(b.root.dataset.theme,'dark');
  }
});
