"""Saved-scene pose validation and CPU-rendered combat previews."""
import bpy
import json
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'artifacts/ranger-combat'
rig=bpy.data.objects['Armature'];scene=bpy.context.scene
def activate(name):
    for owner in [rig,bpy.data.objects['NAC_FixedBowString'].data.shape_keys,bpy.data.objects['NAC_CombatArrow'].data.shape_keys]:
        for t in owner.animation_data.nla_tracks:t.mute=t.name!=name
    scene.frame_set(1);bpy.context.view_layer.update()
def aim(o,target):o.rotation_euler=(Vector(target)-o.location).to_track_quat('-Z','Y').to_euler()
scene.render.engine='CYCLES';scene.cycles.samples=10;scene.cycles.device='CPU'
scene.render.resolution_x=640;scene.render.resolution_y=500;scene.render.resolution_percentage=100
scene.world.color=(.18,.18,.18)
for name,loc,power,size in [('Key',(2,-4,4),450,4),('Fill',(-3,-2,2),220,3),('Rim',(1,3,3),500,3)]:
    d=bpy.data.lights.new(name,'AREA');d.energy=power;d.size=size
    o=bpy.data.objects.new(name,d);scene.collection.objects.link(o);o.location=loc;aim(o,(0,0,1))
bpy.ops.mesh.primitive_plane_add(size=200,location=(0,0,0));ground=bpy.context.object;ground.name='ReviewOnlyGround'
m=bpy.data.materials.new('ReviewGround');m.diffuse_color=(.12,.13,.14,1);ground.data.materials.append(m)
d=bpy.data.cameras.new('ReviewCamera');c=bpy.data.objects.new('ReviewCamera',d);scene.collection.objects.link(c);scene.camera=c
d.type='ORTHO';d.ortho_scale=2.8;c.location=(3,-4,2.5);aim(c,(-.12,0,1.02))
for name,frames in [('Attack_Bow',(1,10,20,31,34,35,49)),('Hit',(1,7,13)),('Death',(1,12,25,37,49))]:
    activate(name)
    for frame in frames:
        scene.frame_set(frame);scene.render.filepath=str(OUT/f'{name}-{frame:02d}.png');bpy.ops.render.render(write_still=True)
print('RANGER_COMBAT_KEY_RENDERS_OK')
report={}
for name,end in [('Attack_Bow',49),('Hit',13),('Death',49)]:
    activate(name);rows=[]
    for f in range(1,end+1):
        scene.frame_set(f);bpy.context.view_layer.update()
        root=rig.matrix_world@rig.pose.bones['root'].head
        assert root.length<1e-5
        rows.append({'frame':f,'head':list(rig.matrix_world@rig.pose.bones['Head'].head),
            'left_hand':list(rig.matrix_world@rig.pose.bones['hand_l'].head),
            'right_hand':list(rig.matrix_world@rig.pose.bones['hand_r'].head),
            'draw':bpy.data.objects['NAC_FixedBowString'].data.shape_keys.key_blocks['Draw'].value,
            'arrow_hidden':bpy.data.objects['NAC_CombatArrow'].data.shape_keys.key_blocks['Hidden'].value})
    report[name]=rows
(OUT/'pose-validation.json').write_text(json.dumps(report,indent=2))
scene.render.resolution_x=448;scene.render.resolution_y=350;scene.cycles.samples=6
for name,end in [('Attack_Bow',49),('Hit',13),('Death',49)]:
    activate(name)
    for frame in range(1,end+1):
        scene.frame_set(frame);scene.render.filepath=str(OUT/f'preview-{name}-{frame:02d}.png');bpy.ops.render.render(write_still=True)
print('RANGER_COMBAT_PREVIEWS_OK')
