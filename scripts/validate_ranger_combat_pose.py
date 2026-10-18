"""Evaluate every combat frame in Blender; no render-only geometry is saved."""
import bpy,json
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[1];rig=bpy.data.objects['Armature'];scene=bpy.context.scene
string=bpy.data.objects['NAC_FixedBowString'];arrow=bpy.data.objects['NAC_CombatArrow']
owners=[rig,string.data.shape_keys,arrow.data.shape_keys]
meshes=[o for o in bpy.data.objects if o.type=='MESH' and o.name.startswith(('Male_Ranger','NAC_'))]
def activate(name):
    for owner in owners:
        for t in owner.animation_data.nla_tracks:t.mute=t.name!=name
def vertices(ob,dg):
    ev=ob.evaluated_get(dg);me=ev.to_mesh();v=[ev.matrix_world@p.co for p in me.vertices];ev.to_mesh_clear();return v
report={};last=None
for name,end in [('Attack_Bow',49),('Hit',13),('Death',49)]:
    activate(name);mins=[];contact=[];hold=[]
    for frame in range(1,end+1):
        scene.frame_set(frame);bpy.context.view_layer.update();dg=bpy.context.evaluated_depsgraph_get()
        assert (rig.matrix_world@rig.pose.bones['root'].head).length<1e-5
        vs=[v for o in meshes if o!=arrow for v in vertices(o,dg)];minimum=min(v.z for v in vs);mins.append(minimum)
        if name=='Death':assert minimum>-.00001,(frame,minimum)
        if name=='Attack_Bow' and 25<=frame<=34:
            midpoint=sum(vertices(string,dg)[6:12],Vector())/6;nock=sum(vertices(arrow,dg)[:6],Vector())/6
            contact.append((midpoint-nock).length)
        if name=='Death' and frame>=43:
            current=[rig.matrix_world@p.head for p in rig.pose.bones]
            if last:hold.append(max((a-b).length for a,b in zip(current,last)))
            last=current
    report[name]={'frames':end,'minimum_height':min(mins),'root_in_place':True}
    if contact:
        report[name]['maximum_nock_string_gap_metres']=max(contact)
        assert max(contact)<.005,contact
    if hold:
        assert max(hold)<1e-6;report[name]['final_pose_max_motion']=max(hold)
(ROOT/'artifacts/ranger-combat/geometry-pose-validation.json').write_text(json.dumps(report,indent=2))
print('COMBAT_POSE_GEOMETRY_OK',report)
