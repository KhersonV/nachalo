import bpy,sys
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'artifacts/ranger-walk';scene=bpy.context.scene
tag=Path(bpy.data.filepath).stem.replace('nachalo_ranger_walk_','')
for ob in bpy.data.objects:
    if ob.animation_data:
        for t in ob.animation_data.nla_tracks:t.mute=t.name!='Walk'
    if ob.type=='MESH' and ob.data.shape_keys and ob.data.shape_keys.animation_data:
        for t in ob.data.shape_keys.animation_data.nla_tracks:t.mute=t.name!='Walk'
def aim(o,p):o.rotation_euler=(Vector(p)-o.location).to_track_quat('-Z','Y').to_euler()
scene.render.engine='CYCLES';scene.cycles.samples=8;scene.cycles.device='CPU'
scene.render.resolution_x=480;scene.render.resolution_y=480;scene.render.resolution_percentage=100;scene.world.color=(.18,.18,.18)
for name,pos,power in [('Key',(2,-4,4),500),('Fill',(-3,-2,2),300)]:
 d=bpy.data.lights.new(name,'AREA');d.energy=power;d.size=4;o=bpy.data.objects.new(name,d);scene.collection.objects.link(o);o.location=pos;aim(o,(0,0,1))
bpy.ops.mesh.primitive_plane_add(size=200,location=(0,0,0))
d=bpy.data.cameras.new('Review');o=bpy.data.objects.new('Review',d);scene.collection.objects.link(o);o.location=(3,-1,1.9);aim(o,(0,-.03,.91));d.type='ORTHO';d.ortho_scale=2.25;scene.camera=o
for f in (1,5,9,13,17,21,25,29):
 scene.frame_set(f);scene.render.filepath=str(OUT/f'{tag}-{f:02d}.png');bpy.ops.render.render(write_still=True)
