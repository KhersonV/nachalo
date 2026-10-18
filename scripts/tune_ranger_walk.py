"""Change only Walk on a copy of the combat master. Blender 5.2."""
import ast,bpy,json,math,sys
from pathlib import Path
from mathutils import Matrix,Quaternion,Vector
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'artifacts/ranger-walk';OUT.mkdir(exist_ok=True)
args=sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else []
amp=float(args[0]) if args else 2.5;drop=float(args[1]) if len(args)>1 else .13
stance=.54
tag=f'a{amp:g}' if args else 'tuned';rig=bpy.data.objects['Armature'];rig_inv=rig.matrix_world.inverted();scene=bpy.context.scene
tree=ast.parse((ROOT/'scripts/build_ranger_animated.py').read_text())
helpers=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name in ('update','head','tail','aim','rotate_world','solve_limb')]
exec(compile(ast.Module(body=helpers,type_ignores=[]),'<walk-ik>','exec'))
for t in rig.animation_data.nla_tracks:t.mute=t.name!='Walk'
poses={};rotations={}
for f in range(1,34):
    scene.frame_set(f);update();poses[f]={p.name:p.matrix_basis.copy() for p in rig.pose.bones}
    rotations[f]={s:(rig.matrix_world@rig.pose.bones['foot_'+s].matrix).to_3x3() for s in ('l','r')}
track=next(t for t in rig.animation_data.nla_tracks if t.name=='Walk');rig.animation_data.nla_tracks.remove(track)
a=bpy.data.actions.new('Walk_Tuned');rig.animation_data.action=a
rows=[];previous={}
for f in range(1,34):
    scene.frame_set(f)
    for p in rig.pose.bones:p.matrix_basis=poses[f][p.name]
    update();phase=2*math.pi*(f-1)/32
    p=rig.pose.bones['pelvis'];m=p.matrix.copy();m.translation+=rig_inv.to_3x3()@Vector((0,0,-drop));p.matrix=m;update()
    lifts={};targets={}
    for side,offset,x in [('l',0,.115),('r',.5,-.115)]:
        u=((f-1)/32+offset)%1
        if u<stance:
            y=-.033-.18*amp+.36*amp*u/stance;lift=0
        else:
            s=(u-stance)/(1-stance);h=3*s*s-2*s*s*s
            # Match backward stance velocity at both swing endpoints.
            tangent=.36*amp*(1-stance)/stance
            y=-.033+.18*amp-.36*amp*h+tangent*(2*s*s*s-3*s*s+s)
            lift=.11*math.sin(math.pi*s)**2
        targets[side]=Vector((x,y,.0865+lift));lifts[side]=lift
        solve_limb('thigh_'+side,'calf_'+side,targets[side],(x,-1,.50))
        foot=rig.pose.bones['foot_'+side];m=rotations[f][side].to_4x4();m.translation=head(foot.name);foot.matrix=rig_inv@m;update()
    for _ in range(4):
        dg=bpy.context.evaluated_depsgraph_get();ob=bpy.data.objects['Male_Ranger_Feet_Boots'].evaluated_get(dg);mesh=ob.to_mesh()
        vs=[ob.matrix_world@v.co for v in mesh.vertices];ob.to_mesh_clear()
        for side,x in [('l',.115),('r',-.115)]:
            minimum=min(v.z for v in vs if (v.x>0 if side=='l' else v.x<0))
            target=head('foot_'+side)+Vector((0,0,.002+lifts[side]-minimum))
            rotation=(rig.matrix_world@rig.pose.bones['foot_'+side].matrix).to_3x3()
            solve_limb('thigh_'+side,'calf_'+side,target,(x,-1,.5))
            m=rotation.to_4x4();m.translation=head('foot_'+side);rig.pose.bones['foot_'+side].matrix=rig_inv@m;update()
    for p in rig.pose.bones:
        if p.name in previous and p.rotation_quaternion.dot(previous[p.name])<0:p.rotation_quaternion.negate()
        p.keyframe_insert('rotation_quaternion',frame=f,group=p.name);p.keyframe_insert('location',frame=f,group=p.name)
    previous={p.name:p.rotation_quaternion.copy() for p in rig.pose.bones}
    dg=bpy.context.evaluated_depsgraph_get();ob=bpy.data.objects['Male_Ranger_Feet_Boots'].evaluated_get(dg);mesh=ob.to_mesh()
    minimum=min((ob.matrix_world@v.co).z for v in mesh.vertices);ob.to_mesh_clear()
    rows.append({'frame':f,'minimum_boot_height':minimum,'foot_l':list(head('foot_l')),'foot_r':list(head('foot_r')),
      'pelvis':list(head('pelvis'))})
for layer in a.layers:
    for strip in layer.strips:
        for bag in strip.channelbags:
            for fc in bag.fcurves:
                for k in fc.keyframe_points:k.interpolation='LINEAR'
rig.animation_data.action=None;t=rig.animation_data.nla_tracks.new();t.name='Walk';st=t.strips.new('Walk',1,a);st.extrapolation='NOTHING'
for t in rig.animation_data.nla_tracks:t.mute=t.name!='Idle'
scene.frame_set(1);update()
name='nachalo_ranger_walk_'+tag
bpy.ops.wm.save_as_mainfile(filepath=str(ROOT/'assets-source/characters/ranger'/(name+'.blend')))
owners=[rig]+[o.data.shape_keys for o in bpy.data.objects if o.type=='MESH' and o.data.shape_keys]
for owner in owners:
    for t in owner.animation_data.nla_tracks:t.mute=False
for o in bpy.context.selected_objects:o.select_set(False)
rig.select_set(True)
for o in bpy.data.objects:
    if o.type=='MESH' and o.name.startswith(('Male_Ranger','NAC_')):o.select_set(True)
bpy.context.view_layer.objects.active=rig
bpy.ops.export_scene.gltf(filepath=str(ROOT/'public/assets/nachalo/characters/ranger'/('ranger_walk_'+tag+'.glb')),
 export_format='GLB',use_selection=True,export_animations=True,export_animation_mode='NLA_TRACKS',export_force_sampling=True,
 export_frame_range=False,export_skins=True,export_all_influences=True,export_extras=True)
report={'amplitude_multiplier':amp,'pelvis_lowering':drop,'stance_fraction':stance,'authored_speed':.36*amp/(32/24*stance),'frames':rows}
(OUT/(tag+'-poses.json')).write_text(json.dumps(report,indent=2));print('WALK_TUNED_OK',tag,min(r['minimum_boot_height'] for r in rows))
