const I18N = {zh: {
    subtitle:'多 VPS 带宽聚合代理', stopped:'已停止', running:'运行中',
    nodesLabel:'节点', online:'在线',
    tabDashboard:'仪表盘', tabNodes:'节点管理', tabLogs:'日志', tabConfig:'配置',
    totalNodes:'总节点', activeLabel:'在线',downSpeed:'下行速率',upSpeed:'上行速率',totalDown:'总下行',totalUp:'总上行', disabledLabel:'已禁用',
    nodeStatus:'节点状态',
    nameCol:'名称', typeCol:'类型', serverCol:'服务器',
    statusCol:'状态', activeConnsCol:'活跃', enabledCol:'启用',
    downTraffic:'下行流量',
    portCol:'端口', passwordCol:'密码', actionsCol:'操作',
    addNode:'+ 添加节点', refresh:'刷新', resetTraffic:'重置流量',
    saveConfigBtn:'保存配置', reloadBtn:'重载',
    addNodeTitle:'添加节点', editNodeTitle:'编辑节点',
    nameLabel:'名称', typeLabel:'类型', serverLabel:'服务器',
    portLabel:'端口', passwordLabel:'密码', sniLabel:'SNI',
    cipherLabel:'加密方式', uuidLabel:'UUID',
    downMbpsLabel:'下行带宽 (Mbps)', upMbpsLabel:'上行带宽 (Mbps)',
    addNodeBtn:'添加节点', editNodeBtn:'保存', cancelBtn:'取消',
    nodeUpdated:'节点已更新', nodeAdded:'节点已添加',
    nodeDeleted:'节点已删除', configSaved:'配置已保存',
    configReloaded:'配置已重载',
    deleteConfirm:'确定要删除此节点吗？',
    onlineStatus:'在线', offlineStatus:'离线',
    failedLoadDashboard:'加载失败: ',
    failedLoadNodes:'加载节点失败: ',
    failedLoadConfig:'加载配置失败: ',
    edit:'编辑', delete:'删除', todo:'需重启 rally run 生效',
    clearBtn:'清空', autoScroll:'自动滚动',
    toggleOn:'开启', toggleOff:'关闭', nodeToggled:'节点开关已切换',
  },
  en: {
    subtitle:'Multi-VPS Bandwidth Aggregation', stopped:'Stopped', running:'Running',
    nodesLabel:'Nodes', online:'online',
    tabDashboard:'Dashboard', tabNodes:'Nodes', tabLogs:'Logs', tabConfig:'Config',
    totalNodes:'Total Nodes', activeLabel:'Active',downSpeed:'Down',upSpeed:'Up',totalDown:'Total Down',totalUp:'Total Up', disabledLabel:'Disabled',
    nodeStatus:'Node Status',
    nameCol:'Name', typeCol:'Type', serverCol:'Server',
    statusCol:'Status', activeConnsCol:'Active', enabledCol:'Enabled',
    downTraffic:'Down',
    portCol:'Port', passwordCol:'Password', actionsCol:'Actions',
    addNode:'+ Add Node', refresh:'Refresh', resetTraffic:'Reset Traffic',
    saveConfigBtn:'Save Config', reloadBtn:'Reload',
    addNodeTitle:'Add Node', editNodeTitle:'Edit Node',
    nameLabel:'Name', typeLabel:'Type', serverLabel:'Server',
    portLabel:'Port', passwordLabel:'Password', sniLabel:'SNI',
    cipherLabel:'Cipher', uuidLabel:'UUID',
    downMbpsLabel:'Down Mbps', upMbpsLabel:'Up Mbps',
    addNodeBtn:'Add Node', editNodeBtn:'Save', cancelBtn:'Cancel',
    nodeUpdated:'Node updated', nodeAdded:'Node added',
    nodeDeleted:'Node deleted', configSaved:'Config saved',
    configReloaded:'Config reloaded',
    deleteConfirm:'Delete this node?',
    onlineStatus:'Online', offlineStatus:'Offline',
    failedLoadDashboard:'Failed to load: ',
    failedLoadNodes:'Failed to load nodes: ',
    failedLoadConfig:'Failed to load config: ',
    edit:'Edit', delete:'Delete', todo:'Restart rally run',
    clearBtn:'Clear', autoScroll:'Auto Scroll',
    toggleOn:'On', toggleOff:'Off', nodeToggled:'Node toggled',
  },
};

let currentLang = localStorage.getItem('rally_lang') || 'zh';

function t(k){return I18N[currentLang]?.[k]||I18N.en[k]||k}

function applyLang(lang){
  currentLang=lang;
  document.querySelectorAll('[data-i18n]').forEach(el=>el.textContent=t(el.dataset.i18n));
  const c=document.getElementById('nodeCount');
  if(c) renderStatusBackends(c.textContent);
  localStorage.setItem('rally_lang',lang);
}

function switchLang(lang){
  applyLang(lang);
  const a=document.querySelector('.tab.active');
  if(a){const t=a.dataset.tab;if(t==='dashboard')loadDashboard();if(t==='nodes')loadNodes();}
}

const API={
  async getConfig(){const r=await fetch('/api/config');if(!r.ok)throw new Error(await r.text());return r.json()},
  async saveConfig(c){const r=await fetch('/api/config',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(c)});if(!r.ok)throw new Error(await r.text());return r.json()},
  async getStatus(){const r=await fetch('/api/status');if(!r.ok)throw new Error(await r.text());return r.json()},
  async reload(){const r=await fetch('/api/reload',{method:'POST'});if(!r.ok)throw new Error(await r.text());return r.json()},
  async getRawConfig(){const r=await fetch('/api/config/raw');if(!r.ok)throw new Error(await r.text());return r.text()},
  async saveRawConfig(y){const r=await fetch('/api/config/raw',{method:'PUT',headers:{'Content-Type':'text/plain'},body:y});if(!r.ok)throw new Error(await r.text());return r.json()},
  async getStats(){const r=await fetch('/api/stats');if(!r.ok)throw new Error(await r.text());return r.json()},
  async getLogs(){const r=await fetch('/api/logs');if(!r.ok)throw new Error(await r.text());return r.json()},
  async toggleNode(name,enabled){const r=await fetch('/api/node/toggle',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,enabled})});if(!r.ok)throw new Error(await r.text());return r.json()},
  async resetStats(){const r=await fetch('/api/stats/reset',{method:'POST'});if(!r.ok)throw new Error(await r.text());return r.json()},
};

let configCache=null,logAutoScroll=true,logStreamActive=false;

function toast(msg,type='success'){
  const el=document.getElementById('toast');
  el.textContent=msg;el.className=`toast ${type} show`;
  clearTimeout(el._timer);
  el._timer=setTimeout(()=>el.classList.remove('show'),3000);
}

document.querySelectorAll('.tab').forEach(tab=>{
  tab.addEventListener('click',()=>{
    document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c=>c.classList.remove('active'));
    tab.classList.add('active');
    document.getElementById('tab-'+tab.dataset.tab).classList.add('active');
    if(tab.dataset.tab==='dashboard')loadDashboard();
    if(tab.dataset.tab==='nodes')loadNodes();
    if(tab.dataset.tab==='logs')loadLogs();
    if(tab.dataset.tab==='config')loadConfigEditor();
  });
});

function onTypeChange(){
  const t=document.getElementById('nodeType').value;
  document.getElementById('rowPassword').style.display=(t==='vless')?'none':'';
  document.getElementById('rowSNI').style.display=(t==='hysteria2'||t==='trojan'||t==='vless')?'':'none';
  document.getElementById('rowCipher').style.display=(t==='ss')?'':'none';
  document.getElementById('rowUUID').style.display=(t==='vless')?'':'none';
  document.getElementById('rowDownMbps').style.display=(t==='hysteria2')?'':'none';
  document.getElementById('rowUpMbps').style.display=(t==='hysteria2')?'':'none';
  document.getElementById('nodePassword').required=(t!=='vless');
  document.getElementById('nodeUUID').required=(t==='vless');
}

function showNodeForm(index){
  const isEdit=index!==undefined;
  document.getElementById('editIndex').value=isEdit?index:'';
  document.getElementById('modalTitle').textContent=isEdit?t('editNodeTitle'):t('addNodeTitle');
  document.getElementById('btnSubmitNode').textContent=isEdit?t('editNodeBtn'):t('addNodeBtn');
  document.getElementById('nodeForm').reset();
  document.getElementById('nodeType').value='hysteria2';onTypeChange();
  if(isEdit&&configCache&&configCache.vps){
    const n=configCache.vps[index];
    document.getElementById('nodeName').value=n.name||'';
    document.getElementById('nodeType').value=n.type||'hysteria2';
    document.getElementById('nodeServer').value=n.server||'';
    document.getElementById('nodePort').value=n.port||'';
    document.getElementById('nodePassword').value=isMaskedPassword(n.password)?'':(n.password||'');
    document.getElementById('nodePassword').placeholder=isMaskedPassword(n.password)?'Leave blank to keep existing password':'Auth password';
    document.getElementById('nodeSNI').value=n.sni||'';
    document.getElementById('nodeCipher').value=n.cipher||'AEAD_CHACHA20_POLY1305';
    document.getElementById('nodeUUID').value=n.uuid||'';
    document.getElementById('nodeDownMbps').value=n.down_mbps||'';
    document.getElementById('nodeUpMbps').value=n.up_mbps||'';onTypeChange();
  }
  document.getElementById('nodeModal').style.display='flex';
}

function closeNodeForm(){document.getElementById('nodeModal').style.display='none'}

async function submitNode(e){
  e.preventDefault();
  const t=document.getElementById('nodeType').value;
  try{
    const cfg=await API.getConfig(),idx=document.getElementById('editIndex').value;
    const existing=idx!==''&&cfg.vps?cfg.vps[parseInt(idx)]:null;
    const node=existing?{...existing}:{};
    node.name=document.getElementById('nodeName').value;
    node.type=t;
    node.server=document.getElementById('nodeServer').value;
    node.port=parseInt(document.getElementById('nodePort').value);
    const password=document.getElementById('nodePassword').value;
    if(t!=='vless'&&password)node.password=password;
    if(t!=='vless'&&!password&&!existing)delete node.password;
    if(t==='hysteria2'||t==='trojan'||t==='vless')node.sni=document.getElementById('nodeSNI').value||undefined;else delete node.sni;
    if(t==='ss')node.cipher=document.getElementById('nodeCipher').value;else delete node.cipher;
    if(t==='vless')node.uuid=document.getElementById('nodeUUID').value;else delete node.uuid;
    if(t==='hysteria2'){
      const dm=document.getElementById('nodeDownMbps').value,um=document.getElementById('nodeUpMbps').value;
      if(dm)node.down_mbps=parseInt(dm);else delete node.down_mbps;
      if(um)node.up_mbps=parseInt(um);else delete node.up_mbps;
    }else{
      delete node.down_mbps;delete node.up_mbps;
    }
    if(idx!=='')cfg.vps[parseInt(idx)]=node;else{cfg.vps=cfg.vps||[];cfg.vps.push(node)}
    await API.saveConfig(cfg);configCache=cfg;
    toast(idx!==''?t('nodeUpdated'):t('nodeAdded'));closeNodeForm();loadNodes();
  }catch(err){toast(err.message,'error')}
}

async function deleteNode(index){
  if(!confirm(t('deleteConfirm')))return;
  try{const cfg=await API.getConfig();cfg.vps.splice(index,1);await API.saveConfig(cfg);configCache=cfg;toast(t('nodeDeleted'));loadNodes()}catch(err){toast(err.message,'error')}
}

document.getElementById('nodeModal').addEventListener('click',e=>{if(e.target===e.currentTarget)closeNodeForm()});

// ─── Dashboard ───────────────────────────────────────────────────────────────

async function toggleNode(name,checked){
  try{
    await API.toggleNode(name, checked);
    toast(t('nodeToggled'));
    loadDashboard();
  }catch(err){toast(err.message,'error')}
}

async function resetTraffic(){
  try{
    await API.resetStats();
    toast('Traffic counters reset');
    loadDashboard();
  }catch(err){toast(err.message,'error')}
}

async function loadDashboard(){
  try{
    const status=await API.getStatus();
    const total=status.backends?status.backends.length:0;
    let active=0,disabled=0;
    status.backends.forEach(b=>{if(!b.enabled)disabled++;else if(b.connected)active++});
    document.getElementById('statTotal').textContent=total;
    document.getElementById('statActive').textContent=active;
    document.getElementById('statDisabled').textContent=disabled;
    document.getElementById('nodeCount').textContent=active;
    renderStatusBackends(active);
    const pe=document.getElementById('statusProxy');
    renderProxyStatus(pe, active>0);
    
    // Build name→stats map
    let statsMap={};
    try{
      const st=await API.getStats();
      st.forEach(s=>{statsMap[s.name]=s});
    }catch(_){}
    
    // Render table
    const tb=document.querySelector('#dashboardTable tbody');tb.innerHTML='';
    let aggDownRate=0,aggUpRate=0,aggDownTotal=0,aggUpTotal=0;
    status.backends.forEach(b=>{
      const en=b.enabled!==false;
      const s=statsMap[b.name];
      const downBytes=s?s.write_total:0;
      const upBytes=s?s.read_total:0;
      const downRate=s?s.write_bps:0;
      const upRate=s?s.read_bps:0;
      aggDownRate+=downRate;aggUpRate+=upRate;
      aggDownTotal+=downBytes;aggUpTotal+=upBytes;
      
      const tr=document.createElement('tr');
      appendTextCell(tr,b.name);
      appendTagCell(tr,b.type||'-',`tag-${safeClass(b.type||'unknown')}`);
      appendTextCell(tr,b.server||'-');
      appendTagCell(tr,en&&b.connected?t('onlineStatus'):t('offlineStatus'),en&&b.connected?'tag-online':'tag-offline');
      appendTextCell(tr,b.active||0);
      appendTextCell(tr,formatBps(downRate),'mono green');
      appendTextCell(tr,formatBytes(downBytes),'mono accent');
      const toggleCell=document.createElement('td');
      const label=document.createElement('label');label.className='switch';
      const input=document.createElement('input');input.type='checkbox';input.checked=en;
      const slider=document.createElement('span');slider.className='slider';
      label.append(input,slider);toggleCell.appendChild(label);tr.appendChild(toggleCell);
      input.addEventListener('change',()=>toggleNode(b.name,input.checked));
      tb.appendChild(tr);
    });
    
    // Update aggregate stats
    const e1=document.getElementById("statDownRate");if(e1)e1.textContent=formatBps(aggDownRate);
    const e2=document.getElementById("statUpRate");if(e2)e2.textContent=formatBps(aggUpRate);
    const e3=document.getElementById("statDownTotal");if(e3)e3.textContent=formatBytes(aggDownTotal);
    const e4=document.getElementById("statUpTotal");if(e4)e4.textContent=formatBytes(aggUpTotal);
  }catch(err){toast(t('failedLoadDashboard')+err.message,'error')}
}

// ─── Nodes ───────────────────────────────────────────────────────────────────

async function loadNodes(){
  try{
    configCache=await API.getConfig();
    const vps=configCache.vps||[],tb=document.querySelector('#nodeTable tbody');tb.innerHTML='';
    vps.forEach((n,i)=>{
      const tr=document.createElement('tr');
      let pw=n.password||"";const pwDisplay=pw?pw.slice(0,1)+"••••"+pw.slice(-1):"";
      const nameTd=document.createElement('td');
      const strong=document.createElement('strong');strong.textContent=n.name||'';
      nameTd.appendChild(strong);tr.appendChild(nameTd);
      appendTagCell(tr,n.type||'hysteria2',`tag-${safeClass(n.type||'hysteria2')}`);
      appendTextCell(tr,n.server||'');
      appendTextCell(tr,n.port||'');
      appendTextCell(tr,pwDisplay,'mono muted');
      const actions=document.createElement('td');
      const editBtn=document.createElement('button');editBtn.className='btn-icon';editBtn.type='button';editBtn.textContent=t('edit');
      const deleteBtn=document.createElement('button');deleteBtn.className='btn-icon btn-danger';deleteBtn.type='button';deleteBtn.textContent=t('delete');
      actions.append(editBtn,deleteBtn);tr.appendChild(actions);
      editBtn.addEventListener('click',()=>showNodeForm(i));
      deleteBtn.addEventListener('click',()=>deleteNode(i));
      tb.appendChild(tr);
    });
  }catch(err){toast(t('failedLoadNodes')+err.message,'error')}
}

// ─── Logs ────────────────────────────────────────────────────────────────────

async function loadLogs(){
  try{
    const logs=await API.getLogs(),c=document.getElementById('logContent');c.innerHTML='';
    logs.forEach(e=>appendLogEntry(c,e));
    if(logAutoScroll){document.getElementById('logContainer').scrollTop=document.getElementById('logContainer').scrollHeight}
    if(!logStreamActive){logStreamActive=true;startLogStream()}
  }catch(err){toast(t('failedLoadDashboard')+err.message,'error')}
}

function appendLogEntry(c,e){
  const d=document.createElement('div');d.className='log-entry';
  const time=document.createElement('span');time.className='log-time';time.textContent=e.time||'';
  const level=document.createElement('span');level.className=`log-level log-level-${safeClass(e.level||'info')}`;level.textContent=e.level||'';
  const msg=document.createElement('span');msg.className='log-msg';msg.textContent=e.message||'';
  d.append(time,level,msg);
  c.appendChild(d);
}

function startLogStream(){
  const es=new EventSource('/api/logs?mode=stream'),c=document.getElementById('logContent');
  es.onmessage=e=>{try{const entry=JSON.parse(e.data);appendLogEntry(c,entry);if(logAutoScroll){document.getElementById('logContainer').scrollTop=document.getElementById('logContainer').scrollHeight}}catch(_){}};
  es.onerror=()=>{es.close();logStreamActive=false;setTimeout(()=>{if(document.getElementById('tab-logs').classList.contains('active'))loadLogs()},2000)};
}

function clearLogs(){document.getElementById('logContent').innerHTML=''}

function toggleLogStream(){logAutoScroll=!logAutoScroll;document.getElementById('btnLogStream').style.opacity=logAutoScroll?'1':'0.5';if(logAutoScroll){document.getElementById('logContainer').scrollTop=document.getElementById('logContainer').scrollHeight}}

// ─── Config ─────────────────────────────────────────────────────────────────

async function loadConfigEditor(){try{document.getElementById('configEditor').value=await API.getRawConfig()}catch(err){toast(t('failedLoadConfig')+err.message,'error')}}
async function saveConfig(){try{await API.saveRawConfig(document.getElementById('configEditor').value);toast(t('configSaved'))}catch(err){toast(err.message,'error')}}
async function reloadConfig(){try{await API.reload();toast(t('configReloaded')+' — '+t('todo'));loadDashboard()}catch(err){toast(err.message,'error')}}

function formatBps(bps){if(bps<1024)return bps.toFixed(0)+' B/s';if(bps<1048576)return(bps/1024).toFixed(1)+' KB/s';return(bps/1048576).toFixed(2)+' MB/s';}
function formatBytes(b){if(b<1024)return b+' B';if(b<1048576)return(b/1024).toFixed(1)+' KB';if(b<1073741824)return(b/1048576).toFixed(1)+' MB';return(b/1073741824).toFixed(2)+' GB';}

function renderStatusBackends(count){
  const el=document.getElementById('statusBackends');
  el.textContent='';
  el.append(document.createTextNode(`${t('nodesLabel')}: `));
  const n=document.createElement('span');n.id='nodeCount';n.textContent=count;
  el.append(n,document.createTextNode(` ${t('online')}`));
}
function renderProxyStatus(el,running){
  el.textContent='';
  el.append(document.createTextNode('● Proxy: '));
  const s=document.createElement('span');s.className=running?'online':'offline';s.textContent=running?t('running'):t('stopped');
  el.appendChild(s);
}
function appendTextCell(tr,value,kind){
  const td=document.createElement('td');td.textContent=value;
  if(kind==='mono green')td.className='cell-mono cell-green';
  if(kind==='mono accent')td.className='cell-mono cell-accent';
  if(kind==='mono muted')td.className='cell-mono cell-muted';
  tr.appendChild(td);
  return td;
}
function appendTagCell(tr,value,extraClass){
  const td=document.createElement('td');
  const span=document.createElement('span');span.className=`tag ${extraClass}`;span.textContent=value;
  td.appendChild(span);tr.appendChild(td);
  return td;
}
function safeClass(s){return String(s||'').replace(/[^a-zA-Z0-9_-]/g,'-')}
function isMaskedPassword(s){return typeof s==='string'&&(s==='****'||s.includes('****'))}

// ─── Init ────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded',()=>{
  document.getElementById('langSwitch').addEventListener('change',e=>switchLang(e.target.value));
  document.getElementById('btnRefreshDashboard').addEventListener('click',loadDashboard);
  document.getElementById('btnResetTraffic').addEventListener('click',resetTraffic);
  document.getElementById('btnAddNode').addEventListener('click',()=>showNodeForm());
  document.getElementById('btnRefreshNodes').addEventListener('click',loadNodes);
  document.getElementById('btnClearLogs').addEventListener('click',clearLogs);
  document.getElementById('btnLogStream').addEventListener('click',toggleLogStream);
  document.getElementById('btnSaveConfig').addEventListener('click',saveConfig);
  document.getElementById('btnReloadConfig').addEventListener('click',reloadConfig);
  document.getElementById('btnCloseNodeModal').addEventListener('click',closeNodeForm);
  document.getElementById('btnCancelNode').addEventListener('click',closeNodeForm);
  document.getElementById('nodeType').addEventListener('change',onTypeChange);
  document.getElementById('nodeForm').addEventListener('submit',submitNode);
  document.getElementById('langSwitch').value=currentLang;
  applyLang(currentLang);loadDashboard();
  setInterval(()=>{if(document.getElementById('tab-dashboard').classList.contains('active'))loadDashboard()},5000);
});
