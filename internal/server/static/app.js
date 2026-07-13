var API='',builderNodes=[],builderEdges=[],selectedNodeId=null,nodePositions={};
var templates=[],_canvasInited=false,_connectFrom=null,_instancesPollTimer=null;

function api(p,o){return fetch(API+p,{headers:{'Content-Type':'application/json'},...o}).then(function(r){return r.status===204?{}:r.json().catch(function(){return{};});});}

// ===== Toast Notification System =====
function showToast(msg, type){
  type=type||'info';
  var container=document.getElementById('toast-container');
  if(!container){container=document.createElement('div');container.id='toast-container';container.style.cssText='position:fixed;top:70px;right:20px;z-index:9999;display:flex;flex-direction:column;gap:8px;pointer-events:none';document.body.appendChild(container);}
  var icons={success:'✓',error:'✗',warning:'⚠',info:'ⓘ'};
  var toast=document.createElement('div');
  toast.className='toast toast-'+type;
  toast.innerHTML='<span class="toast-icon">'+(icons[type]||'')+'</span><span>'+msg+'</span>';
  toast.style.pointerEvents='auto';
  container.appendChild(toast);
  setTimeout(function(){toast.classList.add('toast-remove');setTimeout(function(){if(toast.parentNode)toast.parentNode.removeChild(toast);},250);},3000);
}
// ===== Custom Confirm Dialog =====
var _confirmCallback=null;
function showConfirm(msg, cb, okLabel){
  document.getElementById('confirm-message').textContent=msg;
  document.getElementById('confirm-icon').innerHTML='&#9888;';
  var okBtn=document.getElementById('confirm-ok-btn');
  okBtn.textContent=okLabel||'Confirm';
  _confirmCallback=cb;
  openModal('confirm-dialog');
}
function closeConfirm(val){
  closeModal('confirm-dialog');
  if(_confirmCallback){_confirmCallback(val);_confirmCallback=null;}
}

function showPage(n){
  document.querySelectorAll('.page').forEach(function(p){p.classList.remove('active');});
  document.querySelectorAll('nav a').forEach(function(a){a.classList.remove('active');});
  var el=document.getElementById('page-'+n);if(el)el.classList.add('active');
  var ln=document.querySelector('nav a[data-page="'+n+'"]');if(ln)ln.classList.add('active');
  if(n==='templates')loadTemplates();if(n==='instances')loadInstances();
  // Stop instance poll timer when leaving instances page
  if(n!=='instances'&&_instancesPollTimer){clearInterval(_instancesPollTimer);_instancesPollTimer=null;}
}
function closeModal(id){document.getElementById(id).classList.remove('active');}
function openModal(id){document.getElementById(id).classList.add('active');}

// ===== Builder =====
function openBuilder(existingData){
  nodePositions={};builderNodes=[];builderEdges=[];selectedNodeId=null;_canvasInited=false;_connectFrom=null;
  _startType='User Input';_cronExpr='';
  document.getElementById('add-popup').classList.remove('active');
  if(existingData){
    _startType=existingData.start_type||'User Input';
    _cronExpr=existingData.cron_expr||'';
    (existingData.nodes||[]).forEach(function(n){builderNodes.push(JSON.parse(JSON.stringify(n)));});
    (existingData.edges||[]).forEach(function(e){builderEdges.push(JSON.parse(JSON.stringify(e)));});
    document.getElementById('builder-title').textContent='Edit: '+esc(existingData.name);
  } else {
    _startType='User Input';_cronExpr='';
    document.getElementById('builder-title').textContent='New Workflow';
  }
  renderCanvas();showPage('builder');
  setTimeout(initCanvas,100);setTimeout(centerView,200);
}
function closeBuilder(){
  _editingTemplateId=null;
  showPage('templates');
}
function toggleRight(){document.getElementById('builder-right').classList.toggle('collapsed');}
function toggleAddPopup(){document.getElementById('add-popup').classList.toggle('active');}

function addNode(type){
  document.getElementById('add-popup').classList.remove('active');
  var n=builderNodes.filter(function(x){return x.type===type;}).length+1;
  var node={id:type+'_'+n,type:type};
  if(type==='call'){node.webhook_url='https://httpbin.org/post';node.method='POST';node.body_type='json';}
  if(type==='condition')node.expression='state._global.status == ok';
  if(type==='code'){node.language='js';node.code='// input data available as data\nreturn data';}
  if(type==='extractor'){node.description='Extract data from uploaded file';node.extract_prompt='Extract key information from this file';node.llm_profile='';}
  if(type==='agent')node.agent_config={profile:'',system_prompt:'You are a helpful assistant.',tools:[],max_turns:10};
  _ghostNode=node;
}

function placeNode(x,y){
  if(!_ghostNode)return;
  _ghostNode=null;
}

var _ghostNode=null,_connectEdgeType='flow';

function renderCanvas(){
  var el=document.getElementById('canvas-nodes'),svg=document.getElementById('canvas-svg');
  var pos=calcLayout();
  // START node
  var html='<div class="node'+(selectedNodeId==='_start'?' selected':'')+'" style="left:'+pos._start.x+'px;top:'+pos._start.y+'px;min-width:140px;border-color:#059669;" onclick="selectNodeById(\'_start\')" data-id="_start">';
  html+='<span class="node-type-icon" style="background:#059669;"><svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="2"><path d="M5 3l12 7-12 7V3z"/></svg></span>';
  html+='<span class="node-name" style="font-weight:600;">START</span>';
  html+='<span style="font-size:0.65rem;color:#94a3b8;cursor:pointer;margin-left:auto;" onclick="event.stopPropagation();toggleStartTypePopup()">'+_startType+' &#9660;</span>';
  html+='<div id="start-type-popup" class="add-popup" style="top:0px;left:100%;margin-left:8px;min-width:120px;">'+startTypes.map(function(t){return'<div class="add-popup-item" onclick="event.stopPropagation();changeStartType(\''+t+'\')">'+t+'</div>';}).join('')+'</div>';
  html+='<span class="node-port output" style="right:-11px;" onmousedown="event.stopPropagation();event.preventDefault();_connectFrom=\'_start\';_connectEdgeType=\'flow\';"></span>';
  html+='</div>';
  // User nodes
  var hcolors={call:'#3b82f6',agent:'#8b5cf6',condition:'#f59e0b',code:'#10b981',extractor:'#06b6d4'};
  var nodeIcons={call:'<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="1.5"><path d="M12 4l-4 12M8 8l-4 4 4 4M12 8l4 4-4 4"/></svg>',agent:'<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="1.5"><circle cx="10" cy="6" r="3"/><path d="M4 18c0-3.3 2.7-6 6-6s6 2.7 6 6"/></svg>',condition:'<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="1.5"><path d="M10 2l7 8-7 8-7-8z"/></svg>',code:'<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="1.5"><path d="M5 2h10l3 3v13H5V2z"/><path d="M8 8l-2 2 2 2M12 8l2 2-2 2"/></svg>',extractor:'<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="1.5"><circle cx="10" cy="10" r="7"/><path d="M10 6v8M6 10h8"/></svg>'};
  builderNodes.forEach(function(n){
    var p=pos[n.id];if(!p)return;
    var sel=selectedNodeId===n.id?' selected':'';
    var color=hcolors[n.type]||'#64748b';
    html+='<div class="node'+sel+'" style="left:'+p.x+'px;top:'+p.y+'px;border-color:'+color+';" data-id="'+n.id+'">';
    html+='<span class="node-type-icon" style="background:'+color+';">'+(nodeIcons[n.type]||'')+'</span>';
    html+='<span class="node-name">'+esc(n.id)+'</span>';
    html+='<span class="node-status-circle" style="border:2px solid #94a3b8;"></span>';
    html+='<span class="node-port input" title="Connect input"></span>';
    html+='<span class="node-port output" title="Drag to connect" onmousedown="event.stopPropagation();event.preventDefault();_connectFrom=\''+n.id+'\';_connectEdgeType=\'flow\';"></span>';
    if(n.type==='agent'||n.type==='extractor'){html+='<span class="node-port data" title="Connect data" onmousedown="event.stopPropagation();event.preventDefault();_connectFrom=\''+n.id+'\';_connectEdgeType=\'data\';"></span>';}
    html+='</div>';
  });
  el.innerHTML=html;
  drawConnections(pos);
  if(selectedNodeId&&selectedNodeId!=='_start'){var s=el.querySelector('.node[data-id="'+selectedNodeId+'"]');if(s)s.classList.add('selected');}
}

var _startType='User Input';
var startTypes=['User Input','Schedule'];
var _cronExpr='';

function toggleStartTypePopup(){var p=document.getElementById('start-type-popup');if(p)p.classList.toggle('active');}
function changeStartType(v){_startType=v;document.getElementById('start-type-popup').classList.remove('active');renderCanvas();showProperties();}

function calcLayout(){
  var children={};builderNodes.forEach(function(n){children[n.id]=[];});children['_start']=[];
  builderEdges.forEach(function(e){if(e.edge_type==='data')return;if(children[e.from])children[e.from].push(e.to);});
  var levels={},queue=[],visited=new Set();
  (children['_start']||[]).forEach(function(c){queue.push({id:c,depth:0});});
  var hasIncoming={};builderEdges.forEach(function(e){if(e.edge_type!=='data')hasIncoming[e.to]=true;});
  builderNodes.forEach(function(n){if(!hasIncoming[n.id]&&!levels[n.id]&&levels[n.id]!==0)queue.push({id:n.id,depth:0});});
  while(queue.length){
    var cur=queue.shift();if(visited.has(cur.id))continue;visited.add(cur.id);
    levels[cur.id]=cur.depth;
    (children[cur.id]||[]).forEach(function(c){if(!visited.has(c))queue.push({id:c,depth:cur.depth+1});});
  }
  var byLevel={};Object.keys(levels).forEach(function(id){var lv=levels[id];if(!byLevel[lv])byLevel[lv]=[];byLevel[lv].push(id);});
  var pos={},GAP=220,GAPY=100;
  // START node at left, then levels to the right
  pos._start=nodePositions['_start']||{x:60,y:260};
  Object.keys(byLevel).forEach(function(lv){
    var ids=byLevel[lv],startY=260-(ids.length-1)*GAPY/2;
    ids.forEach(function(id,i){
      if(nodePositions[id])pos[id]=nodePositions[id];
      else pos[id]={x:(parseInt(lv)+1)*GAP+60,y:startY+i*GAPY};
    });
  });
  return pos;
}

function drawConnections(pos){
  var svg=document.getElementById('canvas-svg'),paths=[];
  builderEdges.forEach(function(e){
    var fp=pos[e.from],tp=pos[e.to];if(!fp||!tp)return;
    var fromNode=document.querySelector('.node[data-id="'+e.from+'"]');
    var w=e.from==='_start'?140:(fromNode?fromNode.offsetWidth:172);
    var x1=e.from==='_start'?fp.x+w:fp.x,y1=fp.y+24,x2=tp.x,y2=tp.y+24,cx=(x1+x2)/2;
    var d='M'+x1+','+y1+' C'+cx+','+y1+' '+cx+','+y2+' '+x2+','+y2;
    var isData=e.edge_type==='data';
    var stroke=isData?'#8b5cf6':'#94a3b8';
    var dash=isData?'6,4':'8,4';
    var marker=isData?'':'marker-end="url(#arrow)"';
    paths.push({d:d,from:e.from,to:e.to,stroke:stroke,dash:dash,marker:marker});
  });
  var h='<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10" fill="#94a3b8"/></marker></defs>';
  paths.forEach(function(p){h+='<path d="'+p.d+'" stroke="'+p.stroke+'" stroke-width="2" fill="none" stroke-dasharray="'+p.dash+'" '+p.marker+' style="cursor:pointer;pointer-events:auto;" onclick="editEdgeType(\''+p.from+'\',\''+p.to+'\')"/>';});
  svg.innerHTML=h;
}

function centerView(){
  var c=document.getElementById('builder-canvas');if(!c)return;
  // Center on the nodes area (left side of canvas)
  c.scrollLeft=0;c.scrollTop=Math.max(0,200-c.clientHeight/2);
}

// ===== Canvas Interactions =====
var _ghostNode=null;
function initCanvas(){
  if(_canvasInited)return;_canvasInited=true;
  var c=document.getElementById('builder-canvas');if(!c)return;
  var d={pan:false,drag:false,node:null,ox:0,oy:0,sx:0,sy:0,sl:0,st:0};
  var ghost=document.createElement('div');
  ghost.className='node';ghost.style.position='absolute';ghost.style.display='none';ghost.style.zIndex=100;ghost.style.opacity='0.7';ghost.style.pointerEvents='none';
  ghost.innerHTML='<span class="node-type-icon" style="background:#64748b;"><svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="#fff" stroke-width="2"><circle cx="10" cy="10" r="7"/><path d="M10 7v6M7 10h6"/></svg></span><span class="node-name" style="color:#94a3b8;">Click to place</span>';
  document.getElementById('canvas-viewport').appendChild(ghost);

  c.onmousedown=function(e){
    if(e.target.closest('.add-popup')||e.target.closest('.sidebar')||e.target.closest('#builder-right')||e.target.closest('.right-panel'))return;
    var n=e.target.closest('.node');
    d.sx=e.clientX;d.sy=e.clientY;d.sl=c.scrollLeft;d.st=c.scrollTop;
    if(n&&!e.target.closest('.node-port')){
      var id=n.getAttribute('data-id');if(!id)return;
      d.node={el:n,id:id};var r=n.getBoundingClientRect();d.ox=e.clientX-r.left;d.oy=e.clientY-r.top;n.style.zIndex=100;
    }else if(!n){d.pan=true;c.style.cursor='grabbing';}
  };
  document.onmousemove=function(e){
    if(_ghostNode){
      var cr=c.getBoundingClientRect();var x=e.clientX-cr.left+c.scrollLeft,y=e.clientY-cr.top+c.scrollTop;
      ghost.style.left=(x-86)+'px';ghost.style.top=(y-20)+'px';ghost.style.display='block';
      var gicons={call:'<svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="#fff" stroke-width="1.5"><path d="M12 4l-4 12M8 8l-4 4 4 4M12 8l4 4-4 4"/></svg>',agent:'<svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="#fff" stroke-width="1.5"><circle cx="10" cy="6" r="3"/><path d="M4 18c0-3.3 2.7-6 6-6s6 2.7 6 6"/></svg>',condition:'<svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="#fff" stroke-width="1.5"><path d="M10 2l7 8-7 8-7-8z"/></svg>',code:'<svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="#fff" stroke-width="1.5"><path d="M5 2h10l3 3v13H5V2z"/><path d="M8 8l-2 2 2 2M12 8l2 2-2 2"/></svg>',extractor:'<svg viewBox="0 0 20 20" width="14" height="14" fill="none" stroke="#fff" stroke-width="1.5"><circle cx="10" cy="10" r="7"/><path d="M10 6v8M6 10h8"/></svg>'};
      ghost.innerHTML='<div class="node" style="padding:8px 12px;display:flex;align-items:center;gap:8px;border:3px solid '+(hcolors[_ghostNode.type]||'#64748b')+';"><span class="node-type-icon" style="background:'+(hcolors[_ghostNode.type]||'#64748b')+';">'+(gicons[_ghostNode.type]||'')+'</span><span class="node-name">'+_ghostNode.id+'</span><span class="node-status-circle" style="border:2px solid #94a3b8;"></span></div>';
      return;
    }
    if(d.pan){c.scrollLeft=d.sl-(e.clientX-d.sx);c.scrollTop=d.st-(e.clientY-d.sy);return;}
    if(!d.node)return;
    if(!d.drag&&(Math.abs(e.clientX-d.sx)>3||Math.abs(e.clientY-d.sy)>3))d.drag=true;
    if(!d.drag)return;
    var cr=c.getBoundingClientRect();var x=e.clientX-cr.left-d.ox+c.scrollLeft,y=e.clientY-cr.top-d.oy+c.scrollTop;
    d.node.el.style.left=x+'px';d.node.el.style.top=y+'px';
    nodePositions[d.node.id]={x:x,y:y};drawConnections(calcLayout());
  };
  document.onmouseup=function(e){
    if(_ghostNode){
      var cr=c.getBoundingClientRect();var x=e.clientX-cr.left+c.scrollLeft-86,y=e.clientY-cr.top+c.scrollTop-20;
      _ghostNode.x=x;_ghostNode.y=y;
      builderNodes.push(_ghostNode);
      nodePositions[_ghostNode.id]={x:x,y:y};
      selectedNodeId=_ghostNode.id;
      _ghostNode=null;ghost.style.display='none';
      d.pan=false;d.node=null;d.drag=false;
      renderCanvas();showProperties();
      return;
    }
    if(d.pan){d.pan=false;c.style.cursor='grab';}
    if(d.node){d.node.el.style.zIndex='';if(!d.drag)selectNodeById(d.node.id);d.node=null;d.drag=false;}
    document.getElementById('start-type-popup').classList.remove('active');
    document.getElementById('add-popup').classList.remove('active');
  };
  // Connection drag
  var cl=document.getElementById('connect-line');
  document.addEventListener('mousemove',function(e){
    if(!_connectFrom){cl.innerHTML='';return;}
    var cr=c.getBoundingClientRect();
    var src=document.querySelector('.node[data-id="'+_connectFrom+'"]');
    if(!src){cl.innerHTML='';return;}
    var sr=src.getBoundingClientRect();
    var w=_connectFrom==='_start'?140:src.offsetWidth;
    var x1=sr.left-cr.left+c.scrollLeft+w,y1=sr.top-cr.top+c.scrollTop+24;
    var x2=e.clientX-cr.left+c.scrollLeft,y2=e.clientY-cr.top+c.scrollTop;
    var stroke=_connectEdgeType==='data'?'#8b5cf6':'#0f3460';var dash=_connectEdgeType==='data'?'6,4':'6,4';var marker=_connectEdgeType==='data'?'':'marker-end="url(#arrow)"';cl.innerHTML='<path d="M'+x1+','+y1+' C'+x1+','+((y1+y2)/2)+' '+x2+','+((y1+y2)/2)+' '+x2+','+y2+'" stroke="'+stroke+'" stroke-width="2.5" fill="none" stroke-dasharray="'+dash+'" '+marker+' opacity="0.8"/>';
  });
  document.addEventListener('mouseup',function(e){
    if(!_connectFrom)return;
    var isDataEdge=_connectEdgeType==='data';
    var targetPortClass=isDataEdge?'.node-port.data':'.node-port.input';
    var target=e.target.closest(targetPortClass);
    if(target){
      var nodeEl=target.closest('.node');
      if(nodeEl){
        var id=nodeEl.getAttribute('data-id');
        if(id&&id!==_connectFrom){
          if(!builderEdges.find(function(x){return x.from===_connectFrom&&x.to===id;})){
            var out=builderEdges.filter(function(x){return x.from===_connectFrom&&x.edge_type!=='data';}).length;
            var in_=builderEdges.filter(function(x){return x.to===id&&x.edge_type!=='data';}).length;
            if(out<4&&in_<2){var etype=_connectEdgeType||'flow';builderEdges.push({from:_connectFrom,to:id,edge_type:etype});_connectEdgeType='flow';renderCanvas();showProperties();}
            else showToast('Connection limit: max 4 outgoing, 2 incoming', 'warning');
          }
        }
      }
    }
    _connectFrom=null;_connectEdgeType='flow';cl.innerHTML='';
  });
}

function selectNodeById(id){
  selectedNodeId=id;
  var el=document.getElementById('canvas-nodes');
  if(el){el.querySelectorAll('.node').forEach(function(n){n.classList.remove('selected');});}
  if(id==='_start'){var sn=document.querySelector('.node[data-id="_start"]');if(sn)sn.classList.add('selected');}
  else{var s=el?el.querySelector('.node[data-id="'+id+'"]'):null;if(s)s.classList.add('selected');}
  showProperties();
}

// ===== Properties =====
function showProperties(){
  var panel=document.getElementById('node-properties');
  if(selectedNodeId==='_start'){
    panel.innerHTML='<div style="background:#fff;border-radius:8px;padding:14px;border:1px solid #e2e8f0;"><div style="font-weight:600;margin-bottom:12px;display:flex;align-items:center;gap:8px;"><span style="width:10px;height:10px;border-radius:50%;background:#059669;display:inline-block;"></span> START</div><div class="form-group"><label>Trigger Type</label><select onchange="changeStartType(this.value)">'+startTypes.map(function(t){return'<option value="'+t+'"'+(t===_startType?' selected':'')+'>'+t+'</option>';}).join('')+'</select></div>'+
(_startType==='Schedule'?'<div class="form-group"><label>Cron Expression</label><input value="'+esc(_cronExpr)+'" placeholder="0 9 * * *" onchange="_cronExpr=this.value"><p style="font-size:0.75rem;color:#94a3b8;margin-top:4px;">Format: minute hour day month weekday (e.g. 0 9 * * * = daily at 9:00)</p></div>':'')+
'<p style="font-size:0.8rem;color:#64748b;line-height:1.5;margin-top:8px;">'+
(_startType==='Schedule'?'Configure the cron schedule for automatic execution. The workflow will run at the specified times.':'Configure how this workflow is triggered. The start node defines the input parameters for the workflow.')+'</p></div>';
    return;
  }
  var node=builderNodes.find(function(n){return n.id===selectedNodeId;});
  if(!node){panel.innerHTML='<div class="empty-state" style="padding:16px;"><p style="font-size:0.82rem;">Select a node to edit.</p></div>';return;}
  var colors={call:'#3b82f6',agent:'#8b5cf6',condition:'#f59e0b',code:'#10b981',extractor:'#06b6d4'};
  var h='<div style="background:#fff;border-radius:8px;padding:14px;border:1px solid #e2e8f0;"><div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;"><span style="background:'+(colors[node.type]||'#64748b')+';color:#fff;padding:3px 10px;border-radius:5px;font-size:0.72rem;font-weight:600;">'+(node.type).toUpperCase()+'</span><button class="btn btn-xs btn-danger" onclick="deleteNode(\''+node.id+'\')">Delete</button></div>';
  h+='<div class="form-group"><label>ID</label><input value="'+esc(node.id)+'" onchange="updateNode(\'id\',this.value)"></div>';
  if(node.type==='call'){
  h+='<div class="form-group"><label>Description</label><input value="'+esc(node.description||'')+'" onchange="updateNode(\'description\',this.value)"></div>';
  h+='<div style="display:flex;gap:8px;margin-bottom:14px;"><div style="flex:0 0 100px;"><label>Method</label><select onchange="updateNode(\'method\',this.value)"><option value="GET"'+(node.method==='GET'?' selected':'')+'>GET</option><option value="POST"'+(node.method==='POST'?' selected':'')+'>POST</option><option value="PUT"'+(node.method==='PUT'?' selected':'')+'>PUT</option><option value="DELETE"'+(node.method==='DELETE'?' selected':'')+'>DELETE</option><option value="PATCH"'+(node.method==='PATCH'?' selected':'')+'>PATCH</option></select></div><div style="flex:1;"><label>URL <span style="color:#dc2626;">*</span></label><input value="'+esc(node.webhook_url||'')+'" placeholder="https://api.example.com/endpoint" onchange="updateNode(\'webhook_url\',this.value)"></div></div>';
  h+='<div class="form-group"><label>Body Type</label><select onchange="updateNode(\'body_type\',this.value);showProperties()"><option value="none"'+(node.body_type==='none'||!node.body_type?' selected':'')+'>None</option><option value="raw"'+(node.body_type==='raw'?' selected':'')+'>Raw</option><option value="json"'+(node.body_type==='json'?' selected':'')+'>JSON</option></select></div>';
  if(node.body_type&&node.body_type!=='none'){
    h+='<div class="form-group"><label>Body Content</label><textarea rows="4" onchange="updateNode(\'body_content\',this.value)">'+esc(node.body_content||'')+'</textarea></div>';
  }
}
  if(node.type==='agent'){var a=node.agent_config||{};h+='<div class="form-group"><label>LLM Profile</label><select id="agent-profile-sel" onchange="updateAgent(\'profile\',this.value)"><option value="">Select...</option></select></div><div class="form-group"><label>System Prompt</label><textarea rows="3" onchange="updateAgent(\'system_prompt\',this.value)">'+esc(a.system_prompt||'')+'</textarea></div><div class="form-group"><label>Tools</label><input value="'+esc((a.tools||[]).join(', '))+'" onchange="updateAgent(\'tools\',this.value.split(\',\').map(function(s){return s.trim();}))"></div><div class="form-group"><label>Max Turns</label><input type="number" value="'+(a.max_turns||10)+'" onchange="updateAgent(\'max_turns\',parseInt(this.value)||10)"></div>';setTimeout(function(){var s=document.getElementById('agent-profile-sel');if(s)api('/api/v1/llm/profiles').then(function(ns){if(Array.isArray(ns))s.innerHTML='<option value="">Select...</option>'+ns.map(function(n){return'<option value="'+n+'"'+(a.profile===n?' selected':'')+'>'+n+'</option>';}).join('');});},50);}
  if(node.type==='condition'){h+='<div class="form-group"><label>Expression</label><input value="'+esc(node.expression||'')+'" onchange="updateNode(\'expression\',this.value)"></div>';}
  if(node.type==='extractor'){h+='<div class="form-group"><label>Description</label><input value="'+esc(node.description||'')+'" onchange="updateNode(\'description\',this.value)"></div><div class="form-group"><label>LLM Profile</label><select id="extractor-llm-sel" onchange="updateNode(\'llm_profile\',this.value)"><option value="">Select...</option></select></div><div class="form-group"><label>Extract Prompt</label><textarea rows="3" onchange="updateNode(\'extract_prompt\',this.value)">'+esc(node.extract_prompt||'')+'</textarea></div><div class="form-group"><label>File</label><input type="file" onchange="handleExtractorFile(this)" style="padding:6px;"><div style="font-size:0.75rem;color:#94a3b8;margin-top:4px;">'+(node.file_name||'No file selected')+'</div></div>';setTimeout(function(){var s=document.getElementById('extractor-llm-sel');if(s)api('/api/v1/llm/profiles').then(function(ns){if(Array.isArray(ns))s.innerHTML='<option value="">Select...</option>'+ns.map(function(n){return'<option value="'+n+'"'+(node.llm_profile===n?' selected':'')+'>'+n+'</option>';}).join('');});},50);}
  if(node.type==='code'){h+='<div class="form-group"><label>Language</label><select onchange="updateNode(\'language\',this.value)"><option value="js"'+(node.language==='js'?' selected':'')+'>JavaScript</option><option value="python"'+(node.language==='python'?' selected':'')+'>Python</option></select></div><div class="form-group"><label>Code</label><textarea rows="5" onchange="updateNode(\'code\',this.value)">'+esc(node.code||'')+'</textarea></div>';}
  h+='</div>';panel.innerHTML=h;
}
function updateNode(k,v){var n=builderNodes.find(function(x){return x.id===selectedNodeId;});if(n){n[k]=v;renderCanvas();}}
function updateAgent(k,v){var n=builderNodes.find(function(x){return x.id===selectedNodeId;});if(n){if(!n.agent_config)n.agent_config={};n.agent_config[k]=v;}}
function deleteNode(id){builderNodes=builderNodes.filter(function(n){return n.id!==id;});builderEdges=builderEdges.filter(function(e){return e.from!==id&&e.to!==id;});if(selectedNodeId===id)selectedNodeId=null;renderCanvas();showProperties();}
function changeStartType(v){_startType=v;renderCanvas();showProperties();}

function handleExtractorFile(input){
  var node=builderNodes.find(function(n){return n.id===selectedNodeId;});
  if(!node)return;
  var file=input.files[0];
  if(!file)return;
  var reader=new FileReader();
  reader.onload=function(e){
    var bytes=new Uint8Array(e.target.result);
    var binary='';
    for(var i=0;i<bytes.length;i++)binary+=String.fromCharCode(bytes[i]);
    node.file_content=btoa(binary);
    node.file_name=file.name;
    showProperties();
  };
  reader.readAsArrayBuffer(file);
}

function editEdgeType(fromId,toId){
  var edge=builderEdges.find(function(e){return e.from===fromId&&e.to===toId;});
  if(!edge)return;
  var srcNode=builderNodes.find(function(n){return n.id===fromId;});
  if(!srcNode||(srcNode.type!=='agent'&&srcNode.type!=='extractor')){showToast('Only edges from Agent or Extractor nodes can be changed to Data type.', 'warning');return;}
  var popup=document.getElementById('edge-type-popup');
  if(!popup){
    popup=document.createElement('div');
    popup.id='edge-type-popup';
    popup.className='add-popup';
    popup.style.position='fixed';
    popup.style.zIndex=200;
    document.body.appendChild(popup);
  }
  popup.innerHTML='<div class="add-popup-item" onclick="changeEdgeType(\''+fromId+'\',\''+toId+'\',\''+(edge.edge_type==='data'?'flow':'data')+'\')">Switch to '+(edge.edge_type==='data'?'Flow':'Data')+'</div><div class="add-popup-item" style="color:#94a3b8;font-size:0.75rem;">Current: '+(edge.edge_type||'flow')+'</div>';
  popup.style.left=event.clientX+'px';
  popup.style.top=event.clientY+'px';
  popup.classList.add('active');
  setTimeout(function(){document.addEventListener('click',function closeEdgePopup(e){if(!e.target.closest('#edge-type-popup')){popup.classList.remove('active');document.removeEventListener('click',closeEdgePopup);}});},50);
}
function changeEdgeType(fromId,toId,newType){
  var edge=builderEdges.find(function(e){return e.from===fromId&&e.to===toId;});
  if(edge){edge.edge_type=newType;renderCanvas();}
  var popup=document.getElementById('edge-type-popup');
  if(popup)popup.classList.remove('active');
}

function openSavePopup(){
  var isEditing=!!_editingTemplateId;
  document.getElementById('save-popup-title').textContent=isEditing?'Update Workflow':'Save Workflow';
  document.getElementById('save-popup-btn').textContent=isEditing?'Update':'Save';
  document.getElementById('save-name').value='';
  document.getElementById('save-desc').value='';
  // Pre-fill name/description when editing
  if(isEditing){
    var tmpl=templates.find(function(t){return t.id===_editingTemplateId;});
    if(tmpl){
      document.getElementById('save-name').value=tmpl.name;
      document.getElementById('save-desc').value=tmpl.description||'';
    }
  }
  openModal('save-popup');
}

var localTemplates = [];
var _editingTemplateId = null;

function editTemplate(id){
  api('/api/v1/templates/'+id).then(function(tmpl){
    if(!tmpl||!tmpl.id){showToast('Failed to load template', 'error');return;}
    _editingTemplateId=tmpl.id;
    openBuilder(tmpl);
  });
}

function saveWorkflow(){
  var name=document.getElementById('save-name').value.trim();
  if(!name){showToast('Workflow name is required', 'error');return;}
  if(builderNodes.length===0){showToast('Add at least one node', 'error');return;}
  closeModal('save-popup');
  var edges=builderEdges.map(function(e){var r={from:e.from,to:e.to};if(e.edge_type)r.edge_type=e.edge_type;return r;});
  var body={name:name,description:document.getElementById('save-desc').value.trim(),nodes:builderNodes,edges:edges,start_type:_startType};
  if(_startType==='Schedule'&&_cronExpr)body.cron_expr=_cronExpr;
  if(_editingTemplateId){
    // Update existing template
    body.id=_editingTemplateId;
    api('/api/v1/templates/'+_editingTemplateId,{method:'PUT',body:JSON.stringify(body)}).then(function(r){
      if(r&&r.id){showToast('Workflow updated!', 'success');_editingTemplateId=null;showPage('templates');}
      else{showToast('Update failed: server not available', 'error');}
    });
  } else {
    // Create new template
    api('/api/v1/templates',{method:'POST',body:JSON.stringify(body)}).then(function(r){
      if(r&&r.id){showToast('Workflow saved!', 'success');showPage('templates');}
      else{showToast('Save failed: server not available', 'error');}
    });
  }
}

function testRunWorkflow(){
  if(builderNodes.length===0){showToast('Add at least one node first', 'error');return;}
  _testResults={};
  _testRunning=true;
  _testLog=[];
  showTestPanel('Running workflow...');
  runTestStep('_start',0);
}

var _testResults={},_testRunning=false,_testLog=[];
var _testStep=0;

function runTestStep(nodeId,index){
  if(!_testRunning)return;
  var nextEdges=builderEdges.filter(function(e){return e.from===nodeId&&e.edge_type!=='data';});
  if(nextEdges.length===0&&nodeId!=='_start'){
    _testRunning=false;
    showTestPanel('Workflow completed successfully!');
    return;
  }
  var nodesToProcess=[];
  nextEdges.forEach(function(e){
    var targetNode=builderNodes.find(function(n){return n.id===e.to;});
    if(targetNode)nodesToProcess.push(targetNode);
  });
  if(nodesToProcess.length===0&&nodeId!=='_start'){
    _testRunning=false;
    showTestPanel('Workflow completed successfully!');
    return;
  }
  _testStep++;
  var node=nodesToProcess[0];
  if(!node){
    _testRunning=false;
    showTestPanel('Workflow completed successfully!');
    return;
  }
  updateNodeStatus(node.id,'running');
  setTimeout(function(){
    var success=Math.random()>0.2;
    if(success){
      _testResults[node.id]='success';
      updateNodeStatus(node.id,'success');
      _testLog.push(node.id+': OK');
      runTestStep(node.id,_testStep);
    }else{
      _testResults[node.id]='failed';
      updateNodeStatus(node.id,'failed');
      _testLog.push(node.id+': FAILED');
      _testRunning=false;
      showTestPanel('Workflow failed at node: '+node.id);
    }
  },800);
}

function updateNodeStatus(id,status){
  var icons={running:'<span class="node-status-circle" style="background:#3b82f6;"></span>',success:'<span class="node-status-circle" style="background:#059669;">&#10003;</span>',failed:'<span class="node-status-circle" style="background:#dc2626;">&#10007;</span>'};
  var el=document.getElementById('canvas-nodes');
  if(el){
    var node=el.querySelector('.node[data-id="'+id+'"]')||document.querySelector('.node[data-id="'+id+'"]');
    if(node){
      var icon=node.querySelector('.node-status-circle');
      if(icon)icon.outerHTML=icons[status]||'<span class="node-status-circle" style="border:2px solid #94a3b8;"></span>';
    }
  }
  var svg=document.getElementById('canvas-svg');
  if(svg){
    var paths=svg.querySelectorAll('path');
    paths.forEach(function(p){if(p.getAttribute('marker-end')){p.style.stroke='#94a3b8';p.style.strokeWidth='2';p.style.animation='none';}});
    if(status==='running'){
      builderEdges.filter(function(e){return e.to===id&&e.edge_type!=='data';}).forEach(function(e){
        var pos=calcLayout();
        var fp=pos[e.from],tp=pos[e.to];
        if(fp&&tp){
          var fromNode=document.querySelector('.node[data-id="'+e.from+'"]');
          var w=e.from==='_start'?140:(fromNode?fromNode.offsetWidth:172);
          var x1=e.from==='_start'?fp.x+w:fp.x;
          svg.querySelectorAll('path').forEach(function(p){
            if(p.getAttribute('d')&&p.getAttribute('d').indexOf('M'+x1)!==-1){
              p.style.stroke='#0f3460';p.style.strokeWidth='3';
              p.style.animation='flow 1s linear infinite';
            }
          });
        }
      });
    }
  }
}

function showTestPanel(msg){
  var panel=document.getElementById('test-result');
  if(!panel){
    panel=document.createElement('div');
    panel.id='test-result';
    panel.style.cssText='position:absolute;top:0;right:0;bottom:0;width:280px;z-index:20;background:#fff;border-left:1px solid #e2e8f0;display:flex;flex-direction:column;box-shadow:-4px 0 12px rgba(0,0,0,0.08);animation:slidein 0.25s ease;';
    panel.innerHTML='<div style="display:flex;justify-content:space-between;align-items:center;padding:12px 14px;border-bottom:1px solid #e2e8f0;"><span style="font-size:0.75rem;font-weight:600;color:#64748b;text-transform:uppercase;">Test Result</span><button class="btn btn-xs btn-outline" onclick="this.parentElement.parentElement.remove()">Close</button></div><div id="test-result-body" style="padding:14px;flex:1;overflow-y:auto;font-size:0.85rem;"></div>';
    document.querySelector('#builder-right').parentElement.appendChild(panel);
  }
  var body=document.getElementById('test-result-body');
  if(!body)return;
  var isError=msg.indexOf('failed')>=0||msg.indexOf('FAILED')>=0;
  var icon=isError?'<span style="color:#dc2626;font-size:1.2rem;font-weight:700;">FAILED</span>':'<span style="color:#059669;font-size:1.2rem;font-weight:700;">SUCCESS</span>';
  var color=isError?'#dc2626':'#059669';
  body.innerHTML='<div style="text-align:center;padding:20px 0;">'+icon+'</div><div style="text-align:center;font-weight:600;color:'+color+';margin-bottom:16px;">'+esc(msg)+'</div>';
  if(_testLog.length>0){
    body.innerHTML+='<div style="font-size:0.75rem;font-weight:600;color:#64748b;margin-bottom:8px;text-transform:uppercase;">Execution Log</div>';
    body.innerHTML+='<div style="background:#f8fafc;border-radius:6px;padding:10px;font-family:monospace;font-size:0.78rem;">';
    _testLog.forEach(function(l){
      var c=l.indexOf('FAILED')>=0?'#dc2626':'#059669';
      body.innerHTML+='<div style="color:'+c+';margin-bottom:4px;">'+esc(l)+'</div>';
    });
    body.innerHTML+='</div>';
  }
  body.innerHTML+='<div style="margin-top:16px;padding:10px;background:#f8fafc;border-radius:6px;font-size:0.78rem;color:#64748b;"><strong>Note:</strong> Test run uses simulated execution. In production, actual API calls will be made.</div>';
}

// ===== Other Pages =====
function loadTemplates(){api('/api/v1/templates').then(function(d){templates=Array.isArray(d)?d:[];var t=document.getElementById('template-list');if(templates.length===0){t.innerHTML='<tr><td colspan="6" class="empty-state"><p>No templates yet.</p></td></tr>';return;}t.innerHTML=templates.map(function(x){var st=x.start_type||'User Input';var badge=st==='Schedule'?'<span class="badge" style="background:#fef3c7;color:#92400e;">Schedule</span>':'<span class="badge" style="background:#dbeafe;color:#1d4ed8;">Manual</span>';return'<tr><td><strong>'+esc(x.name)+'</strong><br><span style="font-size:0.75rem;color:#94a3b8;">'+esc(x.description||'')+'</span></td><td>'+badge+'</td><td>'+(x.nodes||[]).length+' nodes</td><td style="font-size:0.8rem;color:#64748b;">'+fmtTime(x.last_run_at)+'</td><td style="font-size:0.8rem;color:#64748b;">'+fmtTime(x.created_at)+'</td><td style="text-align:right;"><button class="btn btn-sm btn-primary" onclick="startInstance(\''+x.id+'\')">Run</button> <button class="btn btn-sm btn-outline" onclick="editTemplate(\''+x.id+'\')">Edit</button> <button class="btn btn-sm btn-danger" onclick="deleteTemplate(\''+x.id+'\')">Delete</button></td></tr>';}).join('');});}
function deleteTemplate(id){
  showConfirm('Delete this template?', function(ok){
    if(!ok)return;
    localTemplates=localTemplates.filter(function(t){return t.id!==id;});
    api('/api/v1/templates/'+id,{method:'DELETE'}).then(function(){loadTemplates();});
  }, 'Delete');
}
function startInstance(id){
  var localTpl=localTemplates.find(function(t){return t.id===id;});
  if(localTpl){
    showToast('Running local workflow: '+localTpl.name, 'info');
    return;
  }
  var d={};api('/api/v1/templates/'+id+'/instances',{method:'POST',body:JSON.stringify({input:d})}).then(function(r){
  if(!r||!r.id){showToast('Failed to start instance', 'error');return;}
  showToast('Instance started', 'success');
  showPage('instances');
});}
function loadInstances(){
  // Build template name map
  var tmplMap={};
  (templates||[]).forEach(function(t){tmplMap[t.id]=t.name;});
  api('/api/v1/instances').then(function(d){
    // If template map is empty, try fetching templates
    if(Object.keys(tmplMap).length===0 && Array.isArray(d) && d.length>0){
      api('/api/v1/templates').then(function(tmpls){
        if(Array.isArray(tmpls)){tmpls.forEach(function(t){tmplMap[t.id]=t.name;});}
        renderInstanceList(d, tmplMap);
      });
    } else {
      renderInstanceList(d, tmplMap);
    }
    // Auto-refresh while any instance is running
    var hasRunning=false;
    if(Array.isArray(d)){d.forEach(function(i){if(i.status==='running'||i.status==='pending')hasRunning=true;});}
    if(hasRunning){
      if(!_instancesPollTimer){_instancesPollTimer=setInterval(loadInstances,3000);}
    } else {
      if(_instancesPollTimer){clearInterval(_instancesPollTimer);_instancesPollTimer=null;}
    }
  });
}
function renderInstanceList(d, tmplMap){
  var l=document.getElementById('instance-list');
  if(!Array.isArray(d)||d.length===0){l.innerHTML='<tr><td colspan="6" class="empty-state"><p>No instances yet.</p></td></tr>';return;}
  l.innerHTML=d.map(function(i){return'<tr><td><code>'+shortId(i.id)+'</code></td><td>'+esc(tmplMap[i.template_id]||i.template_id)+'</td><td><span class="badge badge-'+i.status+'">'+i.status+'</span></td><td>'+(i.current_node_id||'-')+'</td><td>'+fmtTime(i.created_at)+'</td><td><button class="btn btn-xs btn-outline" onclick="showInstance(\''+i.id+'\')">Detail</button> <button class="btn btn-xs btn-danger" onclick="deleteInstance(\''+i.id+'\')">Delete</button></td></tr>';}).join('');
}
function deleteInstance(id){
  showConfirm('Delete this instance?', function(ok){
    if(!ok)return;
    api('/api/v1/instances/'+id,{method:'DELETE'}).then(function(){loadInstances();showToast('Instance deleted', 'success');});
  }, 'Delete');
}
function showInstance(id){
  // Build template name map
  var tmplMap={};
  (templates||[]).forEach(function(t){tmplMap[t.id]=t.name;});
  api('/api/v1/instances/'+id).then(function(i){
    var tmplName=tmplMap[i.template_id]||i.template_id;
    var h='<div class="detail-grid"><div><div class="detail-label">ID</div><div class="detail-value"><code>'+i.id+'</code></div></div><div><div class="detail-label">Status</div><div class="detail-value"><span class="badge badge-'+i.status+'">'+i.status+'</span></div></div><div><div class="detail-label">Template</div><div class="detail-value">'+esc(tmplName)+'</div></div><div><div class="detail-label">Current Node</div><div class="detail-value">'+(i.current_node_id||'-')+'</div></div></div>'+(i.error?'<div style="background:#fef2f2;color:#991b1b;padding:8px 12px;border-radius:6px;font-size:0.82rem;margin-bottom:12px;">'+esc(i.error)+'</div>':'')+
    '<div style="margin-top:8px;"><div class="detail-label">State</div><div class="json-box">'+JSON.stringify(i.state||{},null,2)+'</div></div><div style="margin-top:8px;"><div class="detail-label">Node States</div><div class="json-box">'+JSON.stringify(i.node_states||{},null,2)+'</div></div>';
    document.getElementById('instance-detail').innerHTML=h;
    openModal('modal-instance');
  });
}
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}
function shortId(id){return id?id.substring(0,10)+'...':'-';}
function fmtTime(t){
  if(!t)return '-';
  var d=new Date(t);
  var y=d.getFullYear();
  var M=('0'+(d.getMonth()+1)).slice(-2);
  var day=('0'+d.getDate()).slice(-2);
  var h=('0'+d.getHours()).slice(-2);
  var m=('0'+d.getMinutes()).slice(-2);
  var s=('0'+d.getSeconds()).slice(-2);
  return y+'/'+M+'/'+day+' '+h+':'+m+':'+s;
}
loadTemplates();