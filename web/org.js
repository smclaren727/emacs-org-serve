/* Light read-only Org → HTML renderer, shared by Notes and Journal.
   Handles [[link][desc]], *bold* /italic/ ~code~, headlines and lists;
   strips PROPERTIES drawers and #+keyword lines. Defines globals
   esc / safeUrl / inlineOrg / renderOrg for the pages that include it. */
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}
/* Scheme allowlist for hrefs/srcs: blocks javascript:/data:/vbscript: DOM XSS
   from untrusted captured content. Control chars (tab/newline/etc.) are stripped
   first so they cannot sneak a scheme through the browser's URL normalization. */
function safeUrl(u){
  let r=String(u==null?'':u),t='';
  for(let i=0;i<r.length;i++){const c=r.charCodeAt(i);if(c>31&&c!==127)t+=r[i];}
  t=t.trim();
  if(t===''||t[0]==='#'||t[0]==='/'||t[0]==='.')return t;
  const m=t.match(/^([a-z][a-z0-9+.-]*):/i);
  if(!m)return t;
  return /^(https?|mailto|tel)$/i.test(m[1])?t:'#';
}
function inlineOrg(s){
  s=esc(s);
  s=s.replace(/\[\[([^\]]+?)\]\[([^\]]+?)\]\]/g,(m,u,d)=>`<a href="${safeUrl(u)}" target="_blank" rel="noopener">${d}</a>`);
  s=s.replace(/\[\[([^\]]+?)\]\]/g,(m,u)=>`<a href="${safeUrl(u)}" target="_blank" rel="noopener">${esc(u)}</a>`);
  s=s.replace(/(^|[\s(])\*([^*\s][^*\n]*?)\*(?=[\s).,;:!?'"]|$)/g,'$1<b>$2</b>');
  s=s.replace(/(^|[\s(])\/([^/\s][^/\n]*?)\/(?=[\s).,;:!?'"]|$)/g,'$1<i>$2</i>');
  s=s.replace(/(^|[\s(])[~=]([^~=\s][^~=\n]*?)[~=](?=[\s).,;:!?'"]|$)/g,'$1<code>$2</code>');
  return s;
}
function renderOrg(src){
  src=src.replace(/^[ \t]*:PROPERTIES:[\s\S]*?:END:[ \t]*\n?/gim,'');
  const lines=src.split('\n');
  let html='',listType='';
  const close=()=>{if(listType){html+=`</${listType}>`;listType='';}};
  for(const raw of lines){
    const line=raw.replace(/\s+$/,'');
    if(!line.trim()){close();continue;}
    if(/^#\+/.test(line))continue;
    let m;
    if(m=line.match(/^(\*+)\s+(.*)$/)){
      close();
      const lvl=Math.min(m[1].length+1,6);
      let rest=m[2],todo='';
      const tm=rest.match(/^(TODO|DONE|NEXT|WAITING|WAIT|HOLD|CANCELLED|CANCELED|SOMEDAY)\b\s*/);
      if(tm){todo=`<span class="todo ${tm[1].toLowerCase()}">${tm[1]}</span> `;rest=rest.slice(tm[0].length);}
      rest=rest.replace(/\s+:[\w@#%:]+:\s*$/,'');
      html+=`<h${lvl}>${todo}${inlineOrg(rest)}</h${lvl}>`;continue;
    }
    if(m=line.match(/^\s*[-+]\s+(.*)$/)){
      if(listType!=='ul'){close();html+='<ul>';listType='ul';}
      html+=`<li>${inlineOrg(m[1])}</li>`;continue;
    }
    if(m=line.match(/^\s*\d+[.)]\s+(.*)$/)){
      if(listType!=='ol'){close();html+='<ol>';listType='ol';}
      html+=`<li>${inlineOrg(m[1])}</li>`;continue;
    }
    close();
    html+=`<p>${inlineOrg(line)}</p>`;
  }
  close();
  return html||'<p class="msg">(empty)</p>';
}
