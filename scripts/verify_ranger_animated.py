"""Frame-by-frame evaluation and reproducible CPU renders of the saved asset."""
import bpy
import json
import math
from pathlib import Path
from mathutils import Vector, Quaternion

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'artifacts/ranger-animation'
rig=next(o for o in bpy.data.objects if o.type=='ARMATURE')
scene=bpy.context.scene
def activate(name):
    rig.animation_data.action=None
    for t in rig.animation_data.nla_tracks: t.mute=t.name!=name
    scene.frame_set(1); bpy.context.view_layer.update()
def aim(o,p): o.rotation_euler=(Vector(p)-o.location).to_track_quat('-Z','Y').to_euler()
scene.render.engine='CYCLES'; scene.cycles.samples=12; scene.cycles.device='CPU'
scene.render.resolution_x=512; scene.render.resolution_y=640; scene.render.resolution_percentage=100
scene.world.color=(.18,.18,.18)
# Review-only ground is created after loading the exported source scene.
bpy.ops.mesh.primitive_plane_add(size=200, location=(0,0,0))
ground=bpy.context.object; ground.name='ReviewOnly_Ground'
m=bpy.data.materials.new('ReviewOnly_Ground'); m.diffuse_color=(.11,.12,.13,1)
ground.data.materials.append(m)
for name,loc,power,size in [('Key',(2,-4,4),450,4),('Fill',(-3,-2,2),220,3),('Rim',(1,3,3),500,3)]:
    d=bpy.data.lights.new(name,'AREA'); d.energy=power; d.shape='DISK'; d.size=size
    o=bpy.data.objects.new(name,d); scene.collection.objects.link(o); o.location=loc; aim(o,(0,0,1))
d=bpy.data.cameras.new('ReviewCamera'); c=bpy.data.objects.new('ReviewCamera',d)
scene.collection.objects.link(c); scene.camera=c; d.type='ORTHO'
def render(name,loc,target=(0,0,.95),scale=2.15):
    c.location=loc; aim(c,target); d.ortho_scale=scale
    scene.render.filepath=str(OUT/(name+'.png')); bpy.ops.render.render(write_still=True)
activate('Idle')
render('idle-front',(0,-4,1.05))
render('idle-side',(4,0,1.05))
render('idle-back',(0,4,1.05))
render('face-closeup',(0,-3,1.72),(0,0,1.70),.48)
for sign in (-1,1):
    activate('Idle')
    scene.frame_set(1)
    pose={pb.name:pb.matrix_basis.copy() for pb in rig.pose.bones}
    for t in rig.animation_data.nla_tracks: t.mute=True
    for pb in rig.pose.bones: pb.matrix_basis=pose[pb.name]
    neck=rig.pose.bones['neck_01']; skull=rig.pose.bones['Head']
    neck.rotation_quaternion=neck.rotation_quaternion @ Quaternion((0,1,0),sign*math.radians(5))
    skull.rotation_quaternion=skull.rotation_quaternion @ Quaternion((0,1,0),sign*math.radians(10))
    bpy.context.view_layer.update()
    render('head-turn-'+str(sign),(0,-3,1.72),(0,0,1.70),.48)
activate('Walk')
for frame in (1,5,9,13,17,21,25,29):
    scene.frame_set(frame)
    render('walk-side-%02d'%frame,(4,0,1.05))
    render('walk-front-%02d'%frame,(1.7,-4,1.7))

# Evaluate every saved frame, including the closing duplicate. Inspect evaluated
# boot vertices (not just ankle bones) to detect penetration and detached meshes.
report={}
for name,end in [('Idle',73),('Walk',33)]:
    activate(name); rows=[]; endpoints=[]
    for frame in range(1,end+1):
        scene.frame_set(frame); bpy.context.view_layer.update()
        dg=bpy.context.evaluated_depsgraph_get()
        boot=bpy.data.objects['Male_Ranger_Feet_Boots'].evaluated_get(dg)
        me=boot.to_mesh()
        pts=[boot.matrix_world @ v.co for v in me.vertices]
        boot.to_mesh_clear()
        row={'frame':frame,'boot_min_l':min(p.z for p in pts if p.x>0),
             'boot_min_r':min(p.z for p in pts if p.x<0),
             'root':list(rig.matrix_world @ rig.pose.bones['root'].head)}
        rows.append(row)
        if frame in (1,end):
            endpoints.append({pb.name:rig.matrix_world @ pb.matrix for pb in rig.pose.bones})
    error=max(abs(endpoints[0][n][i][j]-endpoints[1][n][i][j]) for n in endpoints[0] for i in range(4) for j in range(4))
    assert error<1e-5,(name,'loop mismatch',error)
    assert all(max(abs(v) for v in row['root'])<1e-6 for row in rows)
    report[name]={'frames_checked':end,'endpoint_matrix_error':error,'frames':rows}
(OUT/'validation.json').write_text(json.dumps(report,indent=2))
print('RANGER_FRAME_VALIDATION_OK')

# Modest animated preview, generated from the actual saved Walk action.
activate('Walk')
scene.render.resolution_x=384; scene.render.resolution_y=480; scene.cycles.samples=8
for frame in range(1,33):
    scene.frame_set(frame)
    render('preview-%02d'%frame,(2,-4,1.8))
print('RANGER_PREVIEWS_OK')
