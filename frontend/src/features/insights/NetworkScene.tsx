import { useEffect, useRef, useState } from 'react';
import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import type { UsageNetwork } from './network';
import type { Metric } from './timeline';

type Props = { graph: UsageNetwork; metric: Metric; selected: string; onSelect: (id: string) => void; paused: boolean; reset: number; expanded?: boolean; view?: 'perspective'|'front'|'side'|'above' };
export default function NetworkScene({graph,metric,selected,onSelect,paused,reset,expanded=false,view="perspective"}:Props) {
 const host=useRef<HTMLDivElement>(null), selectRef=useRef(onSelect), selectedRef=useRef(selected), pausedRef=useRef(paused);
 const [failed,setFailed]=useState(false);
 useEffect(()=>{selectRef.current=onSelect;selectedRef.current=selected;pausedRef.current=paused;},[onSelect,selected,paused]);
 useEffect(()=>{
  const container=host.current;
  if(!container) return;
  let renderer: THREE.WebGLRenderer;

  // oxlint-disable-next-line react/set-state-in-effect
  try { renderer=new THREE.WebGLRenderer({antialias:true,alpha:true,powerPreference:'low-power'}); } catch { setFailed(true); return; }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio,1.75));
  renderer.setClearColor(0x080c16,0);
  container.appendChild(renderer.domElement);
  const canvas=renderer.domElement;
  canvas.style.cssText='width:100%;height:100%;display:block;touch-action:pan-y;';
  canvas.setAttribute('aria-label','3D usage network. Drag to orbit, pinch to zoom. Use the node selector for keyboard access.');
  canvas.setAttribute('role','img');
  const scene=new THREE.Scene();
  const camera=new THREE.PerspectiveCamera(42,1,0.1,150);
  const controls=new OrbitControls(camera,canvas);
  controls.enableDamping=true; controls.dampingFactor=.075; controls.enablePan=false;
  controls.minDistance=13; controls.maxDistance=55; controls.minPolarAngle=.45; controls.maxPolarAngle=Math.PI-.45;
  controls.touches.ONE=THREE.TOUCH.ROTATE; controls.touches.TWO=THREE.TOUCH.DOLLY_PAN;
  scene.add(new THREE.AmbientLight(0xffffff,2));
  const light=new THREE.PointLight(0xa9d9ff,90);light.position.set(0,5,8);scene.add(light);
  const sphere=new THREE.SphereGeometry(1,24,16);
  const glowCanvas=document.createElement('canvas');glowCanvas.width=128;glowCanvas.height=128;
  const ctx=glowCanvas.getContext('2d')!;
  const gradient=ctx.createRadialGradient(64,64,0,64,64,64);gradient.addColorStop(0,'rgba(255,255,255,.6)');gradient.addColorStop(.2,'rgba(255,255,255,.22)');gradient.addColorStop(1,'rgba(255,255,255,0)');ctx.fillStyle=gradient;ctx.fillRect(0,0,128,128);
  const glowTexture=new THREE.CanvasTexture(glowCanvas);
  const max=Math.max(...graph.nodes.filter(n=>n.kind==='model'||n.kind==='group').map(n=>n.usage[metric]),1);
  const objects=new Map<string,THREE.Mesh<THREE.SphereGeometry,THREE.MeshStandardMaterial>>();
  const labels: {element:HTMLDivElement;position:THREE.Vector3;id:string;kind:string}[]=[];
  graph.nodes.forEach(node=>{
   const radius=node.kind==='total'?.48:node.kind==='source'?.34:.12+Math.cbrt(node.usage[metric]/max)*.27;
   const material=new THREE.MeshStandardMaterial({color:node.color,emissive:node.color,emissiveIntensity:.65,roughness:.25,metalness:.25});
   const mesh=new THREE.Mesh(sphere,material);mesh.position.set(...node.position);mesh.scale.setScalar(radius);mesh.userData.id=node.id;scene.add(mesh);objects.set(node.id,mesh);
   const glow=new THREE.Sprite(new THREE.SpriteMaterial({map:glowTexture,color:node.color,transparent:true,depthWrite:false,blending:THREE.AdditiveBlending,opacity:.7}));glow.scale.setScalar(radius*8);mesh.add(glow);glow.scale.divideScalar(radius);
   const label=document.createElement('div');label.textContent=node.label;
   label.style.cssText=`position:absolute;pointer-events:none;white-space:nowrap;line-height:1.4;background:rgba(8,12,22,.78);border-radius:4px;padding:2px 5px;font-size:${node.kind==='model'||node.kind==='group'?13:14}px;color:${node.kind==='model'||node.kind==='group'?'#d2dbe9':'#e2e8f0'};font-weight:${node.kind==='source'?600:400};text-shadow:0 1px 8px #080c16;transform:translate(-50%,0);`;
   container.appendChild(label);labels.push({element:label,position:mesh.position.clone(),id:node.id,kind:node.kind});
  });
  const flows:{curve:THREE.CubicBezierCurve3;dot:THREE.Mesh;line:THREE.Line;from:string;to:string;offset:number}[]=[];
  graph.edges.forEach((edge,index)=>{
   const a=objects.get(edge.from)!.position,b=objects.get(edge.to)!.position;
   const curve=new THREE.CubicBezierCurve3(a,a.clone().add(new THREE.Vector3(1.7,0,1.2)),b.clone().add(new THREE.Vector3(-1.7,0,1.2)),b);
   const color=objects.get(edge.to)!.material.color;
   const line=new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(48)),new THREE.LineBasicMaterial({color,transparent:true,opacity:.25}));scene.add(line);
   const dot=new THREE.Mesh(sphere,new THREE.MeshBasicMaterial({color,transparent:true,opacity:.9}));dot.scale.setScalar(.045);scene.add(dot);
   flows.push({curve,dot,line,from:edge.from,to:edge.to,offset:index*.173});
  });
  const grid=new THREE.GridHelper(42,42,0x283149,0x172034);grid.position.y=-9;grid.material.transparent=true;grid.material.opacity=.35;scene.add(grid);
  let width=0,height=0;
  const resize=()=>{width=container.clientWidth;height=container.clientHeight;renderer.setSize(width,height,false);camera.aspect=width/height;const distance=Math.max(28,23/camera.aspect,...graph.nodes.map(node=>Math.abs(node.position[1])*3+7));const direction=view==='front'?[0,0,1]:view==='side'?[.8,.1,.65]:view==='above'?[.1,.85,.65]:[.22,.16,1];camera.far=distance*6;controls.maxDistance=Math.max(55,distance*3);camera.position.set(direction[0],direction[1],direction[2]).normalize().multiplyScalar(distance);camera.lookAt(0,0,0);camera.updateProjectionMatrix();controls.target.set(0,0,0);controls.update();};
  const observer=new ResizeObserver(resize);observer.observe(container);resize();
  const raycaster=new THREE.Raycaster(), pointer=new THREE.Vector2();let downX=0,downY=0;
  const down=(event:PointerEvent)=>{downX=event.clientX;downY=event.clientY;};
  const up=(event:PointerEvent)=>{if(Math.hypot(event.clientX-downX,event.clientY-downY)>5)return;const bounds=canvas.getBoundingClientRect();pointer.set((event.clientX-bounds.left)/bounds.width*2-1,-(event.clientY-bounds.top)/bounds.height*2+1);raycaster.setFromCamera(pointer,camera);const hit=raycaster.intersectObjects([...objects.values()],false)[0];if(hit)selectRef.current(hit.object.userData.id as string);};
  const lost=(event:Event)=>{event.preventDefault();setFailed(true);};
  canvas.addEventListener('pointerdown',down);canvas.addEventListener('pointerup',up);canvas.addEventListener('webglcontextlost',lost);
  let frame=0,visible=true,time=0,last=0;
  const visibility=new IntersectionObserver(entries=>{visible=entries[0].isIntersecting;});visibility.observe(container);
  const reduced=window.matchMedia('(prefers-reduced-motion: reduce)');
  const projected=new THREE.Vector3();
  const animate=(now:number)=>{
   frame=requestAnimationFrame(animate);const delta=Math.min((now-last)/1000,.05);last=now;
   if(!visible||document.hidden)return;
   if(!pausedRef.current&&!reduced.matches)time+=delta;
   controls.update();
   flows.forEach(flow=>{const related=selectedRef.current==='total'||flow.from===selectedRef.current||flow.to===selectedRef.current; (flow.line.material as THREE.LineBasicMaterial).opacity=related?.38:.07;flow.dot.visible=related;flow.dot.position.copy(flow.curve.getPoint((time*.12+flow.offset)%1));});
   objects.forEach((mesh,id)=>{mesh.material.emissiveIntensity=id===selectedRef.current?1.8:.55;});
   renderer.render(scene,camera);
   const placed: {x:number;y:number;w:number}[]=[];
   const orderedLabels=[...labels].sort((a,b)=>(a.id===selectedRef.current?-2:a.kind==='model'||a.kind==='group'?1:0)-(b.id===selectedRef.current?-2:b.kind==='model'||b.kind==='group'?1:0));
   orderedLabels.forEach(label=>{projected.copy(label.position).project(camera);const x=(projected.x+1)*width/2,y=(-projected.y+1)*height/2+16;label.element.style.display='block';const w=label.element.offsetWidth;const overlaps=placed.some(p=>Math.abs(p.y-y)<24&&Math.abs(p.x-x)<(p.w+w)/2+6);const show=!overlaps&&projected.z<1&&x-w/2>6&&x+w/2<width-6&&y>0&&y<height-20&&(width>550||label.kind==='source'||label.kind==='total'||label.id===selectedRef.current);if(show)placed.push({x,y,w});label.element.style.display=show?'block':'none';label.element.style.left=`${x}px`;label.element.style.top=`${y}px`;label.element.style.color=label.id===selectedRef.current?'#fff':label.kind==='source'?'#e2e8f0':'#d2dbe9';});
  };
  frame=requestAnimationFrame(animate);
  return()=>{cancelAnimationFrame(frame);observer.disconnect();visibility.disconnect();controls.dispose();canvas.removeEventListener('pointerdown',down);canvas.removeEventListener('pointerup',up);canvas.removeEventListener('webglcontextlost',lost);scene.traverse(object=>{if(object instanceof THREE.Mesh||object instanceof THREE.Line||object instanceof THREE.Sprite){if('geometry' in object)object.geometry.dispose();const materials=Array.isArray(object.material)?object.material:[object.material];materials.forEach(material=>material.dispose());}});glowTexture.dispose();renderer.dispose();canvas.remove();labels.forEach(label=>label.element.remove());};
 },[graph,metric,reset,view]);
 return <div ref={host} className={`relative min-w-0 overflow-hidden ${expanded ? "h-[560px] lg:h-[max(580px,calc(100dvh-300px))]" : "h-[440px] sm:h-[530px]"}`}>{failed&&<div role="status" className="absolute inset-0 z-10 flex items-center justify-center bg-[#080c16] p-8 text-center text-sm text-slate-400">3D rendering is unavailable in this browser. All usage values are available in the node selector and detail panel.</div>}</div>;
}
