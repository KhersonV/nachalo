"""Build a separate Ranger asset on the unchanged Quaternius 65-bone rig.

Run: D:/Blender/blender.exe -b assets-source/characters/ranger/nachalo_ranger_final.blend --python scripts/build_ranger_animated.py
No external services. Original geometry/materials and source files are retained.
"""
import bpy
import json
import math
import warnings
from pathlib import Path
from mathutils import Matrix, Quaternion, Vector
from mathutils.bvhtree import BVHTree

warnings.filterwarnings('ignore', message="'Material.use_nodes'.*", category=DeprecationWarning)

SRC = Path(bpy.data.filepath).resolve()
assert SRC.name == 'nachalo_ranger_final.blend', 'Load the final source, never an output'
ROOT = SRC.parents[3]
OUT = ROOT / 'artifacts/ranger-animation'
OUT.mkdir(parents=True, exist_ok=True)
rig = next(o for o in bpy.data.objects if o.type == 'ARMATURE')
assert len(rig.data.bones) == 65
rig_transform = rig.matrix_world.copy()
rig_inv = rig_transform.inverted()
source_geometry = {o.name: len(o.data.vertices) for o in bpy.data.objects if o.type == 'MESH'}
rig.animation_data_clear()
for pb in rig.pose.bones:
    pb.rotation_mode = 'QUATERNION'
    pb.matrix_basis.identity()
    assert not pb.constraints, 'Unexpected source constraint: ' + pb.name
character = [o for o in bpy.data.objects if o.type == 'MESH' and o.name.startswith(('Male_Ranger', 'NAC_'))]
for o in bpy.data.objects:
    if o.type == 'MESH' and o not in character:
        o.hide_render = True
        o.hide_set(True)

# Preserve the drape and manual offsets, but lift vertices that were embedded
# in the back of the source tunic. Work in canonical rig coordinates.
body=bpy.data.objects['Male_Ranger_Body']
body_bvh=BVHTree.FromPolygons([body.matrix_local @ v.co for v in body.data.vertices],
                            [list(p.vertices) for p in body.data.polygons])
cape=bpy.data.objects['NAC_FixedDrapedCape']
cape_inverse=cape.matrix_local.inverted()
cape_adjusted=0
for v in cape.data.vertices:
    p=cape.matrix_local @ v.co
    hit,normal,index,distance=body_bvh.ray_cast(Vector((p.x,1,p.z)),Vector((0,-1,0)),2)
    if hit is not None and p.y<hit.y+.012:
        p.y=hit.y+.012; v.co=cape_inverse @ p; cape_adjusted+=1
# The source quiver was intersecting the drape. Keep the arrows and both quiver
# pieces together and move the assembly just behind the corrected cloth.
for ob in character:
    if ob.name.startswith(('NAC_FixedQuiver','NAC_FixedArrow')):
        ob.location.y+=.055
print('CAPE_CONTACT_VERTICES_CORRECTED',cape_adjusted)

def material(name, color, roughness=.8):
    m = bpy.data.materials.new(name)
    m.diffuse_color = (*color, 1)
    m.use_nodes = True
    p = next(n for n in m.node_tree.nodes if n.type == 'BSDF_PRINCIPLED')
    p.inputs['Base Color'].default_value = (*color, 1)
    p.inputs['Roughness'].default_value = roughness
    return m

fabric = material('NAC_AnatomicalMask_Graphite', (.035, .044, .050))
skin = material('NAC_EyeOpening_Skin', (.36, .205, .13))
white = material('NAC_Eyes_Ivory', (.40, .37, .30), .45)
iris = material('NAC_Eyes_Hazel', (.095, .065, .028), .3)
pupil = material('NAC_Eyes_Pupil', (.006, .008, .009), .3)
brow = material('NAC_Eyebrows', (.035, .023, .018))

def mesh_object(name, vertices, faces, materials):
    me = bpy.data.meshes.new(name + '_Mesh')
    me.from_pydata(vertices, [], faces)
    me.update()
    for m in materials: me.materials.append(m)
    ob = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(ob)
    ob.parent = rig
    ob.matrix_parent_inverse.identity()
    g = ob.vertex_groups.new(name='Head')
    g.add(list(range(len(vertices))), 1, 'REPLACE')
    mod = ob.modifiers.new('OriginalSkeleton', 'ARMATURE'); mod.object = rig
    for p in me.polygons: p.use_smooth = True
    character.append(ob)
    return ob

# Tailored closed volume: neck, chin, jaw, cheekbones, brow, forehead, crown.
# The source has only hood shell/lining. This is the permitted fitted-mask
# fallback, not a claim to have recovered an original human head.
rings = [
    (1.548,.049,.056,.033), (1.582,.053,.063,.026),
    (1.607,.055,.073,.019), (1.633,.067,.082,.020),
    (1.660,.077,.090,.024), (1.686,.082,.094,.025),
    (1.708,.083,.095,.026), (1.726,.081,.093,.026),
    (1.744,.080,.094,.027), (1.761,.082,.096,.028),
    (1.782,.080,.094,.030), (1.804,.067,.081,.032),
    (1.820,.045,.059,.035), (1.828,.018,.030,.037)]
N = 48
vertices=[]
for z,rx,ry,cy in rings:
    for j in range(N):
        a=2*math.pi*j/N; x=rx*math.sin(a); y=cy-ry*math.cos(a)
        front=max(0,math.cos(a))
        # Project a nose bridge/tip and cheek structure into the aperture.
        nose=.039*math.exp(-((z-1.707)/.032)**2)*math.exp(-(x/.022)**2)*front**8
        cheeks=.008*math.exp(-((z-1.714)/.022)**2)*math.exp(-((abs(x)-.049)/.023)**2)*front**4
        sockets=.006*math.exp(-((z-1.744)/.012)**2)*math.exp(-((abs(x)-.034)/.018)**2)*front**8
        vertices.append((x,y-nose-cheeks+sockets,z))
faces=[]
for k in range(len(rings)-1):
    for j in range(N):
        faces.append((k*N+j,k*N+(j+1)%N,(k+1)*N+(j+1)%N,(k+1)*N+j))
faces += [tuple(range(N-1,-1,-1)), tuple(range((len(rings)-1)*N,len(rings)*N))]
mask=mesh_object('NAC_FittedAnatomicalMask',vertices,faces,[fabric,skin])
for p in mask.data.polygons:
    c=p.center
    # Eye opening exposes skin above the face covering, within the hood.
    if 1.726 < c.z < 1.782 and c.y < -.025 and abs(c.x)<.076:
        p.material_index=1

# Eyes are curved almond patches with eyelid rims; no glowing flat cards.
for side in (-1,1):
    cx=side*.034; zc=1.748
    vs=[(cx,-.0755,zc)]; fs=[]
    steps=24
    for j in range(steps):
        a=2*math.pi*j/steps
        vs.append((cx+.018*math.cos(a),-.0685,zc+.0048*math.sin(a)))
    for j in range(steps): fs.append((0,j+1,(j+1)%steps+1))
    mesh_object('NAC_Eye_'+str(side),vs,fs,[white])
    for name,rad,mat,y in [('Iris',.0053,iris,-.076),('Pupil',.0025,pupil,-.0768)]:
        vs=[(cx,y-.0006,zc)]+[(cx+rad*math.cos(j*2*math.pi/24),y,zc+rad*math.sin(j*2*math.pi/24)) for j in range(24)]
        mesh_object('NAC_'+name+'_'+str(side),vs,[(0,j+1,(j+1)%24+1) for j in range(24)],[mat])
    vs=[]
    for j in range(25):
        a=2*math.pi*j/24
        for radius in (1,1.22):
            vs.append((cx+.018*radius*math.cos(a),-.069-radius*.0005,zc+.0048*radius*math.sin(a)))
    mesh_object('NAC_Eyelids_'+str(side),vs,[(2*j,2*j+1,2*j+3,2*j+2) for j in range(24)],[skin])
    vs=[]
    for j in range(9):
        u=j/8*2-1
        z=1.766+.004*(1-u*u)
        y=-.070+.013*abs(u)
        vs.extend([(cx+.023*u,y,z),(cx+.023*u,y-.0005,z+.0035)])
    mesh_object('NAC_Brow_'+str(side),vs,[(2*j,2*j+1,2*j+3,2*j+2) for j in range(8)],[brow])

def update(): bpy.context.view_layer.update()
def head(name): return rig.matrix_world @ rig.pose.bones[name].head
def tail(name): return rig.matrix_world @ rig.pose.bones[name].tail

def aim(name, target):
    """Minimal swing from the current evaluated direction, respecting bone roll."""
    update()
    pb=rig.pose.bones[name]
    h=head(name); current=tail(name)-h
    q=current.normalized().rotation_difference((Vector(target)-h).normalized())
    world=rig.matrix_world @ pb.matrix
    rot=q.to_matrix() @ world.to_3x3()
    desired=rot.to_4x4(); desired.translation=h
    pb.matrix=rig_inv @ desired
    update()

def rotate_world(name, axis, angle):
    update()
    pb=rig.pose.bones[name]; world=rig.matrix_world @ pb.matrix
    rot=Quaternion(Vector(axis),angle).to_matrix() @ world.to_3x3()
    result=rot.to_4x4(); result.translation=world.translation
    pb.matrix=rig_inv @ result
    update()

def solve_limb(upper,lower,target,pole):
    """Analytic two-link IK, baked directly to the original deform bones."""
    update()
    h=head(upper); t=Vector(target); d=t-h
    a=rig.data.bones[upper].length; b=rig.data.bones[lower].length
    distance=min(d.length,a+b-.0001)
    along=d.normalized()
    p=Vector(pole)-h; p=(p-along*p.dot(along)).normalized()
    x=(a*a-b*b+distance*distance)/(2*distance)
    height=math.sqrt(max(0,a*a-x*x))
    knee=h+along*x+p*height
    aim(upper,knee); aim(lower,t)
    return knee

def reset():
    for pb in rig.pose.bones: pb.matrix_basis.identity()
    # Counter the diagnosed source-object tilt in the root bone's rest basis.
    # Armature object transform and all 65 rest matrices remain unchanged.
    rig.pose.bones['root'].rotation_quaternion=rig_transform.to_quaternion().inverted()
    update()

def pelvis_move(z):
    pb=rig.pose.bones['pelvis']; m=pb.matrix.copy()
    m.translation += rig_inv.to_3x3() @ Vector((0,0,z))
    pb.matrix=m; update()

def arms(phase=0,walking=False):
    # Left wrist stays outboard of the thigh; elbow bends forward.
    sway=math.sin(phase) if walking else 0
    solve_limb('upperarm_l','lowerarm_l',(.355,-.070-.012*sway,1.020),(.55,-.20,1.23))
    aim('hand_l',head('hand_l')+Vector((.06,-.12,-1)))
    solve_limb('upperarm_r','lowerarm_r',(-.265,-.042+.060*sway,.995),(-.40,-.19,1.20))
    aim('hand_r',head('hand_r')+Vector((-.08,-.10,-1)))
    # Curl fingers in the actual hand orientation, using rest roll implicitly.
    for side in ('l','r'):
        for finger in ('index','middle','ring','pinky'):
            for segment in (1,2,3):
                pb=rig.pose.bones[f'{finger}_{segment:02d}_{side}']
                pb.rotation_quaternion=Quaternion((0,0,1),(-1.10 if side=='l' else .18))
    # Oppose the left thumb across the closed fingers instead of leaving it spread.
    thumb_target=head('hand_l')+Vector((.008,.032,-.10))
    for segment in (1,2,3):
        aim(f'thumb_{segment:02d}_l',thumb_target)
    update()

# Recalibrate the preserved bow geometry around its grip so it stays upright in
# the relaxed wrist pose. Its original dimensions and mesh topology are retained.
reset(); arms()
# Also check the relaxed pose: lowering the arms changes the tunic's shoulder
# surface. Correct between the cape's sparse rows with extra cloth clearance.
dg=bpy.context.evaluated_depsgraph_get()
evbody=body.evaluated_get(dg); me=evbody.to_mesh()
posed_body=BVHTree.FromPolygons([evbody.matrix_world @ v.co for v in me.vertices],
                              [list(p.vertices) for p in me.polygons])
evbody.to_mesh_clear()
for v in cape.data.vertices:
    weighted=Matrix(((0,0,0,0),)*4)
    for group in v.groups:
        name=cape.vertex_groups[group.group].name
        if name in rig.pose.bones:
            weighted += (rig.pose.bones[name].matrix @ rig.data.bones[name].matrix_local.inverted())*group.weight
    transform=rig.matrix_world @ weighted @ rig_inv @ cape.matrix_world
    p=transform @ v.co
    hit,_,_,_=posed_body.ray_cast(Vector((p.x,1,p.z)),Vector((0,-1,0)),2)
    if hit is not None and p.y<hit.y+.028:
        p.y=hit.y+.028; v.co=transform.inverted() @ p
grip=bpy.data.objects['NAC_FixedBowGrip']
old_center=sum((v.co for v in grip.data.vertices),Vector())/len(grip.data.vertices)
new_center=head('hand_l')+Vector((.008,.028,-.100))
deform=rig.pose.bones['hand_l'].matrix @ rig.data.bones['hand_l'].matrix_local.inverted()
for ob in character:
    if ob.name in ('NAC_FixedBow','NAC_FixedBowGrip','NAC_FixedBowString'):
        for v in ob.data.vertices:
            desired=new_center+(v.co-old_center)
            v.co=deform.inverted() @ (rig_inv @ desired)
        ob['animation_grip_calibrated']=True

scene=bpy.context.scene; scene.render.fps=24
clips={}; metrics={}
for name,end in [('Idle',73),('Walk',33)]:
    action=bpy.data.actions.new(name); action.use_fake_user=True
    rig.animation_data_create(); rig.animation_data.action=action
    clips[name]=action
    rows=[]
    for frame in range(1,end+1):
        scene.frame_set(frame); reset()
        phase=2*math.pi*(frame-1)/(end-1)
        foot_lifts={}
        if name=='Idle':
            pelvis_move(-.008+.0015*math.sin(phase))
            rotate_world('spine_02',(1,0,0),.0035*math.sin(phase))
            rotate_world('spine_03',(1,0,0),.0045*math.sin(phase))
            for side,x in [('l',.099),('r',-.099)]:
                solve_limb('thigh_'+side,'calf_'+side,(x,.0875,.0865),(x,-1,.5))
                # Restore foot orientation in world space, independent of knee.
                pb=rig.pose.bones['foot_'+side]
                m=rig.data.bones[pb.name].matrix_local.copy(); m.translation=head(pb.name)
                pb.matrix=rig_inv @ m; update()
                foot_lifts[side]=0
            arms()
        else:
            # 32-frame stride, 60% planted stance, 40% smooth swing.
            # Stance travels backward at constant 0.45 m/s for game in-place playback.
            pelvis_move(-.025+.010*math.cos(2*phase))
            rotate_world('pelvis',(0,0,1),.025*math.cos(phase))
            rotate_world('spine_02',(0,0,1),-.020*math.cos(phase))
            rotate_world('spine_03',(1,0,0),.020)
            for side,offset,x in [('l',0,.099),('r',.5,-.099)]:
                u=((frame-1)/(end-1)+offset)%1
                if u<.6:
                    y=-.213+.600*u
                    lift=0
                    # Heel strike settles over the first 12% of stance.
                    pitch=-.10*max(0,1-u/.075)+.14*max(0,(u-.50)/.10)
                else:
                    s=(u-.6)/.4
                    # Hermite endpoints match backward stance velocity.
                    h=3*s*s-2*s*s*s
                    y=.147-.36*h+.24*(2*s*s*s-3*s*s+s)
                    lift=.095*math.sin(math.pi*s)**2
                    pitch=.14*math.cos(math.pi*s)
                solve_limb('thigh_'+side,'calf_'+side,(x,y,.0865+lift),(x,-1,.50))
                pb=rig.pose.bones['foot_'+side]
                footrot=Quaternion((1,0,0),pitch).to_matrix() @ rig.data.bones[pb.name].matrix_local.to_3x3()
                m=footrot.to_4x4(); m.translation=head(pb.name)
                pb.matrix=rig_inv @ m; update()
                foot_lifts[side]=lift
            arms(phase,True)
        # Ground correction uses the actual deformed sole, including the ankle
        # roll and calf weights. This is baked IK, with no runtime constraints.
        for correction in range(3):
            update(); dg=bpy.context.evaluated_depsgraph_get()
            boot=bpy.data.objects['Male_Ranger_Feet_Boots'].evaluated_get(dg)
            me=boot.to_mesh()
            points=[boot.matrix_world @ v.co for v in me.vertices]
            minima={'l':min(p.z for p in points if p.x>0),
                    'r':min(p.z for p in points if p.x<0)}
            boot.to_mesh_clear()
            for side,x in [('l',.099),('r',-.099)]:
                shift=.002+foot_lifts[side]-minima[side]
                pb=rig.pose.bones['foot_'+side]
                rotation=(rig.matrix_world @ pb.matrix).to_3x3()
                target=head(pb.name)+Vector((0,0,shift))
                solve_limb('thigh_'+side,'calf_'+side,target,(x,-1,.50))
                m=rotation.to_4x4(); m.translation=head(pb.name)
                pb.matrix=rig_inv @ m; update()
        rotate_world('neck_01',(0,0,1),.007*math.sin(phase))
        rotate_world('Head',(0,0,1),.009*math.sin(phase))
        for pb in rig.pose.bones:
            # Quaternion sign continuity prevents interpolation taking a long arc.
            if frame>1 and pb.rotation_quaternion.dot(previous[pb.name])<0:
                pb.rotation_quaternion.negate()
            pb.keyframe_insert('rotation_quaternion',frame=frame,group=pb.name)
            pb.keyframe_insert('location',frame=frame,group=pb.name)
        previous={pb.name:pb.rotation_quaternion.copy() for pb in rig.pose.bones}
        rows.append({'frame':frame,'ankle_l':list(head('foot_l')),'ankle_r':list(head('foot_r')),
                     'head':list(head('Head')),'root':list(head('root'))})
    for layer in action.layers:
        for strip in layer.strips:
            for bag in strip.channelbags:
                for fc in bag.fcurves:
                    for key in fc.keyframe_points: key.interpolation='LINEAR'
    metrics[name]=rows

rig.animation_data.action=None
for name,action in clips.items():
    track=rig.animation_data.nla_tracks.new(); track.name=name
    strip=track.strips.new(name,1,action); strip.extrapolation='NOTHING'
    track.mute=(name!='Idle')
scene.frame_start=1; scene.frame_end=73; scene.frame_set(1); update()
assert all(len(bpy.data.objects[n].data.vertices)==count for n,count in source_geometry.items())
assert all(abs(rig.matrix_world[i][j]-rig_transform[i][j])<1e-6 for i in range(4) for j in range(4))
rig['source_author']='Quaternius'
rig['animation_note']='In-place Walk: 0.45 m/s; 24 fps; 32 frames plus duplicate endpoint. Fitted mask fallback.'
for ob in bpy.context.selected_objects: ob.select_set(False)
rig.hide_set(False); rig.select_set(True)
for ob in character: ob.hide_set(False); ob.select_set(True)
bpy.context.view_layer.objects.active=rig
blend=SRC.with_name('nachalo_ranger_animated.blend')
bpy.ops.wm.save_as_mainfile(filepath=str(blend))
# Export both tracks separately; save the blend with only Idle enabled for viewing.
for t in rig.animation_data.nla_tracks: t.mute=False
glb=ROOT/'public/assets/nachalo/characters/ranger/ranger_animated.glb'
bpy.ops.export_scene.gltf(filepath=str(glb),export_format='GLB',use_selection=True,
    export_animations=True,export_animation_mode='NLA_TRACKS',export_force_sampling=True,
    export_frame_range=False,export_skins=True,export_all_influences=True)
(OUT/'build-metrics.json').write_text(json.dumps({'source':str(SRC),'bones':65,
    'source_object_matrix':[list(row) for row in rig_transform],
    'source_mesh_vertex_counts':source_geometry,'clips':metrics},indent=2))
print('RANGER_BUILD_OK',blend,glb)
