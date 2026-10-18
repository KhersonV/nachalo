"""Compare the saved asset against source datablocks without saving either."""
import bpy
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
rig=bpy.data.objects['Armature']
original_objects={o.name:o for o in bpy.data.objects}
with bpy.data.libraries.load(str(ROOT/'assets-source/characters/ranger/nachalo_ranger_final.blend'),link=False) as (a,b):
    requested=list(a.objects)
    b.objects=list(requested)
sources=dict(zip(requested,b.objects))
sr=sources['Armature']
assert len(sr.data.bones)==len(rig.data.bones)==65
error=max(abs(rig.data.bones[bone.name].matrix_local[i][j]-bone.matrix_local[i][j])
          for bone in sr.data.bones for i in range(4) for j in range(4))
assert error==0
assert all((bone.parent.name if bone.parent else None)==
           (rig.data.bones[bone.name].parent.name if rig.data.bones[bone.name].parent else None)
           for bone in sr.data.bones)
assert sr.rotation_mode==rig.rotation_mode
for field in ('location','rotation_quaternion','scale','delta_location','delta_rotation_euler','delta_scale'):
    assert tuple(getattr(sr,field))==tuple(getattr(rig,field)),field
checked=[]
for name,ob in original_objects.items():
    if not name.startswith('Male_Ranger'): continue
    source=sources[name]
    assert len(ob.data.vertices)==len(source.data.vertices)
    assert all((v.co-w.co).length==0 for v,w in zip(ob.data.vertices,source.data.vertices))
    assert [tuple(p.vertices) for p in ob.data.polygons]==[tuple(p.vertices) for p in source.data.polygons]
    assert [[(g.group,g.weight) for g in v.groups] for v in ob.data.vertices]==[[(g.group,g.weight) for g in v.groups] for v in source.data.vertices]
    assert len(ob.data.uv_layers)==len(source.data.uv_layers)
    for uv1,uv2 in zip(ob.data.uv_layers,source.data.uv_layers):
        assert all((x.uv-y.uv).length==0 for x,y in zip(uv1.data,uv2.data))
    assert len(ob.data.materials)==len(source.data.materials)
    checked.append(name)
result={'bones':65,'rest_matrix_error':error,'hierarchy_unchanged':True,
        'armature_object_transform_unchanged':True,'unchanged_original_meshes':checked,
        'geometry_topology_uv_weights_checked':True}
(ROOT/'artifacts/ranger-animation/preservation-validation.json').write_text(json.dumps(result,indent=2))
print('RANGER_PRESERVATION_OK',result)
