"""Non-destructive combat pass on the saved animated Ranger, Blender 5.2."""
import ast
import bpy
import json
import math
from pathlib import Path
from mathutils import Matrix, Quaternion, Vector

src=Path(bpy.data.filepath).resolve()
assert src.name=='nachalo_ranger_animated.blend'
ROOT=src.parents[3]; OUT=ROOT/'artifacts/ranger-combat'; OUT.mkdir(parents=True,exist_ok=True)
rig=bpy.data.objects['Armature']; assert len(rig.data.bones)==65
rig_transform=rig.matrix_world.copy(); rig_inv=rig_transform.inverted()
scene=bpy.context.scene; scene.render.fps=24
character=[o for o in bpy.data.objects if o.type=='MESH' and o.name.startswith(('Male_Ranger','NAC_'))]
source_counts={o.name:len(o.data.vertices) for o in character}
clips={t.name:t.strips[0].action for t in rig.animation_data.nla_tracks}
assert set(clips)=={'Idle','Walk'}
for t in rig.animation_data.nla_tracks: t.mute=t.name!='Idle'
scene.frame_set(1); bpy.context.view_layer.update()
base={p.name:p.matrix_basis.copy() for p in rig.pose.bones}
base_world={p.name:(rig.matrix_world@p.matrix).copy() for p in rig.pose.bones}
rig.animation_data_clear()
# Reuse only the tested world-space IK helpers; do not rebuild or edit Walk.
tree=ast.parse((ROOT/'scripts/build_ranger_animated.py').read_text())
helpers=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name in ('update','head','tail','aim','rotate_world','solve_limb')]
exec(compile(ast.Module(body=helpers,type_ignores=[]),'<ranger-world-ik>','exec'))
def reset():
    for p in rig.pose.bones: p.matrix_basis=base[p.name]
    update()
def hand_rotation(name,rotation):
    m=rotation.to_4x4(); m.translation=head(name); rig.pose.bones[name].matrix=rig_inv@m; update()
left_rotation=Quaternion((0,0,1),-math.pi/2).to_matrix()@base_world['hand_l'].to_3x3()
idle_left_rotation=base_world['hand_l'].to_3x3()
def arm(upper,lower,target,pole): solve_limb(upper,lower,target,pole)
def blend(a,b,t): return Vector(a).lerp(Vector(b),t)
def smooth(t): t=max(0,min(1,t)); return t*t*(3-2*t)

# Replace only the output string by an equivalent 3-ring tube, with a draw morph.
string=bpy.data.objects['NAC_FixedBowString']
old=[v.co.copy() for v in string.data.vertices]
ends=[sum(old[:6],Vector())/6,sum(old[6:],Vector())/6]
cent=(ends[0]+ends[1])/2
verts=[]
for c in (ends[0],cent,ends[1]):
    for j in range(6):
        offset=old[j]-ends[0]
        verts.append(c+offset)
faces=[(r*6+j,r*6+(j+1)%6,(r+1)*6+(j+1)%6,(r+1)*6+j) for r in range(2) for j in range(6)]
faces += [tuple(range(5,-1,-1)),tuple(range(12,18))]
me=bpy.data.meshes.new('NAC_CombatString_Mesh');me.from_pydata(verts,[],faces);me.update()
for m in string.data.materials:me.materials.append(m)
string.data=me;string.vertex_groups.clear();string.vertex_groups.new(name='hand_l').add(list(range(18)),1,'REPLACE')
string.shape_key_add(name='Basis'); draw=string.shape_key_add(name='Draw')

# A skinned decorative arrow. Hidden by morph in all other clips; no gameplay.
bone=rig.data.bones['hand_r']; v=[]; f=[]
for distance,radius in ((.035,.004),(.55,.004),(.57,.010),(.61,0)):
    for j in range(6):
        a=j*math.pi/3
        v.append(bone.matrix_local@Vector((radius*math.cos(a),distance,radius*math.sin(a))))
for r in range(3):
    for j in range(6):f.append((r*6+j,r*6+(j+1)%6,(r+1)*6+(j+1)%6,(r+1)*6+j))
f.append(tuple(range(5,-1,-1)))
mesh=bpy.data.meshes.new('NAC_CombatArrow_Mesh');mesh.from_pydata(v,[],f);mesh.update()
mesh.materials.append(bpy.data.objects['NAC_FixedArrow_0'].data.materials[0])
arrow=bpy.data.objects.new('NAC_CombatArrow',mesh);scene.collection.objects.link(arrow)
arrow.parent=rig;arrow.matrix_parent_inverse.identity()
arrow.vertex_groups.new(name='hand_r').add(list(range(len(v))),1,'REPLACE')
arrow.modifiers.new('OriginalSkeleton','ARMATURE').object=rig
arrow.shape_key_add(name='Basis'); hidden=arrow.shape_key_add(name='Hidden')
for p in hidden.data:p.co=bone.matrix_local@Vector((0,.035,0))
hidden.value=1;character.append(arrow)

left_idle=head('hand_l').copy();right_idle=head('hand_r').copy()
left_up=Vector((.12,-.53,1.48)); draw_nock=Vector((-.085,-.055,1.56))
fetch=Vector((.12,.245,1.64))
def set_left(t):
    arm('upperarm_l','lowerarm_l',left_idle.lerp(left_up,t),(.45,-.22,1.47))
    hand_rotation('hand_l',idle_left_rotation.to_quaternion().slerp(left_rotation.to_quaternion(),t).to_matrix())
def string_mid():
    d=rig.pose.bones['hand_l'].matrix@rig.data.bones['hand_l'].matrix_local.inverted()
    return rig.matrix_world@d@rig_inv@string.matrix_world@cent

# Measure the draw morph in the actual raised-bow frame, with original bone roll.
reset();set_left(1)
rest_nock=string_mid()
full_draw=draw_nock-rest_nock
d=rig.pose.bones['hand_l'].matrix@rig.data.bones['hand_l'].matrix_local.inverted()
transform=rig.matrix_world@d@rig_inv@string.matrix_world
for i in range(6,12):draw.data[i].co+=transform.inverted().to_3x3()@full_draw

def attack(frame):
    if frame<=10:
        t=smooth((frame-1)/9);set_left(.08*t)
        right=right_idle.lerp(fetch,t);pole=(-.45,.20,1.55)
    elif frame<=20:
        t=smooth((frame-10)/10);set_left(.08+.92*t)
        right=fetch.lerp(rest_nock+Vector((-.07,.01,.015)),t);pole=(-.45,-.12,1.55)
    elif frame<=34:
        set_left(1);t=smooth((frame-23)/8)
        contact=rest_nock.lerp(draw_nock,t)
        direction=(Vector((left_up.x+.008,left_up.y+.028,left_up.z-.10))-contact).normalized()
        right=contact-direction*.035;pole=(-.48,.16,1.62)
    else:
        t=smooth((frame-36)/13);set_left(1-t)
        contact=draw_nock+Vector((-.04,.05,.02))
        right=contact.lerp(right_idle,t);pole=(-.48,.10,1.5)
    arm('upperarm_r','lowerarm_r',right,pole)
    if 21<=frame<=34: direction=(contact-head('hand_r')).normalized()
    elif 18<=frame<=35: direction=(string_mid()-head('hand_r')).normalized()
    else: direction=Vector((-.10,-.1,-1))
    aim('hand_r',head('hand_r')+direction)
    # Finger curl around the nock. The left fist is inherited from Idle.
    for finger in ('index','middle','ring','pinky'):
        for segment in (1,2,3):
            rig.pose.bones[f'{finger}_{segment:02d}_r'].rotation_quaternion=Quaternion((0,0,1),.7 if frame<36 else .2)
    draw.value=smooth((frame-23)/8) if frame<35 else 0
    hidden.value=0 if 10<=frame<35 else 1
    if frame==49: reset()

def hit(frame):
    if frame in (1,13):
        draw.value=0;hidden.value=1;return
    pulse=math.sin(math.pi*(frame-1)/12)**2
    rotate_world('spine_02',(1,0,0),-.12*pulse)
    rotate_world('spine_03',(1,0,0),-.07*pulse)
    rotate_world('Head',(1,0,0),.055*pulse)
    arm('upperarm_l','lowerarm_l',left_idle+Vector((.015,.025,.045))*pulse,(.45,-.2,1.2))
    arm('upperarm_r','lowerarm_r',right_idle+Vector((-.03,-.07,.08))*pulse,(-.45,-.2,1.2))
    draw.value=0;hidden.value=1

def death(frame):
    t=(frame-1)/42;c=smooth(t/.45);fall=smooth((t-.28)/.62)
    pb=rig.pose.bones['pelvis'];m=pb.matrix.copy();m.translation+=rig_inv.to_3x3()@Vector((0,0,-.26*c));pb.matrix=m;update()
    for side in ('l','r'):
        p=base_world['foot_'+side].translation
        arm('thigh_'+side,'calf_'+side,p,(p.x,-1,.5))
        hand_rotation('foot_'+side,base_world['foot_'+side].to_3x3())
    rotate_world('spine_02',(1,0,0),.17*c)
    arm('upperarm_l','lowerarm_l',left_idle.lerp(Vector((.14,-.20,1.12)),c),(.4,-.35,1.25))
    arm('upperarm_r','lowerarm_r',right_idle.lerp(Vector((-.15,-.28,1.18)),c),(-.4,-.4,1.25))
    pelvis=rig.pose.bones['pelvis'];world=rig.matrix_world@pelvis.matrix
    world=Quaternion((0,1,0),-math.pi/2*fall).to_matrix().to_4x4()@world
    world.translation=Vector((-.12*fall,.043+.025*fall,(.949-.26*c)*(1-fall)+.21*fall))
    pelvis.matrix=rig_inv@world;update()
    draw.value=0;hidden.value=1
    # Bake clearance from all evaluated original meshes, including head/cape/bow.
    dg=bpy.context.evaluated_depsgraph_get();minimum=10
    for ob in character:
        if ob==arrow:continue
        ev=ob.evaluated_get(dg);me=ev.to_mesh()
        minimum=min(minimum,min((ev.matrix_world@v.co).z for v in me.vertices));ev.to_mesh_clear()
    if minimum<.006:
        m=pelvis.matrix.copy();m.translation+=rig_inv.to_3x3()@Vector((0,0,.006-minimum));pelvis.matrix=m;update()

shape_data=[string.data.shape_keys,arrow.data.shape_keys]
shape_actions={}
for name,end,pose in [('Attack_Bow',49,attack),('Hit',13,hit),('Death',49,death)]:
    a=bpy.data.actions.new(name);a.use_fake_user=True;clips[name]=a
    rig.animation_data_create();rig.animation_data.action=a
    for keys in shape_data:
        keys.animation_data_create();ma=bpy.data.actions.new(keys.name+'_'+name);ma.use_fake_user=True
        keys.animation_data.action=ma;shape_actions[(keys.name,name)]=ma
    prev={}
    for frame in range(1,end+1):
        scene.frame_set(frame);reset();pose(frame)
        for pb in rig.pose.bones:
            if pb.name in prev and pb.rotation_quaternion.dot(prev[pb.name])<0:pb.rotation_quaternion.negate()
            pb.keyframe_insert('rotation_quaternion',frame=frame,group=pb.name)
            pb.keyframe_insert('location',frame=frame,group=pb.name)
        prev={p.name:p.rotation_quaternion.copy() for p in rig.pose.bones}
        draw.keyframe_insert('value',frame=frame);hidden.keyframe_insert('value',frame=frame)

# Constant hidden/de-tensioned morphs in the untouched locomotion clips.
for keys in shape_data:
    for name,end in [('Idle',73),('Walk',33)]:
        a=bpy.data.actions.new(keys.name+'_'+name);a.use_fake_user=True;keys.animation_data.action=a
        shape_actions[(keys.name,name)]=a
        for frame in (1,end):
            draw.value=0;hidden.value=1
            key=draw if keys==string.data.shape_keys else hidden
            key.keyframe_insert('value',frame=frame)
for a in list(clips.values())[2:]+list(shape_actions.values()):
    for layer in a.layers:
        for strip in layer.strips:
            for bag in strip.channelbags:
                for fc in bag.fcurves:
                    for k in fc.keyframe_points:k.interpolation='LINEAR'
rig.animation_data.action=None
for name,a in clips.items():
    tr=rig.animation_data.nla_tracks.new();tr.name=name;st=tr.strips.new(name,1,a);st.extrapolation='NOTHING';tr.mute=name!='Idle'
for keys in shape_data:
    keys.animation_data.action=None
    for name in clips:
        tr=keys.animation_data.nla_tracks.new();tr.name=name;st=tr.strips.new(name,1,shape_actions[(keys.name,name)]);st.extrapolation='NOTHING';tr.mute=name!='Idle'
meta={'fps':24,'attackReleaseSeconds':34/24,'attackArrowVisibleSeconds':9/24,
      'durations':{'Idle':3,'Walk':32/24,'Attack_Bow':2,'Hit':.5,'Death':2},'deathHoldFromSeconds':42/24}
rig['combat_release_seconds']=meta['attackReleaseSeconds'];rig['source_author']='Quaternius'
scene.frame_start=1;scene.frame_end=73;scene.frame_set(1);update()
for o in bpy.context.selected_objects:o.select_set(False)
rig.select_set(True)
for o in character:o.hide_set(False);o.select_set(True)
bpy.context.view_layer.objects.active=rig
bpy.ops.wm.save_as_mainfile(filepath=str(src.with_name('nachalo_ranger_combat.blend')))
for owner in [rig]+shape_data:
    for track in owner.animation_data.nla_tracks:track.mute=False
bpy.ops.export_scene.gltf(filepath=str(ROOT/'public/assets/nachalo/characters/ranger/ranger_combat.glb'),
    export_format='GLB',use_selection=True,export_animations=True,export_animation_mode='NLA_TRACKS',
    export_force_sampling=True,export_frame_range=False,export_skins=True,export_all_influences=True,export_extras=True)
(ROOT/'src/data').mkdir(exist_ok=True)
(ROOT/'src/data/rangerCombat.json').write_text(json.dumps(meta,indent=2))
(OUT/'build-metadata.json').write_text(json.dumps({'metadata':meta,'original_vertex_counts':source_counts},indent=2))
print('RANGER_COMBAT_BUILD_OK',meta)
