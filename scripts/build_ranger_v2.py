"""Generate Ranger v2: an improved procedural silhouette on the original 17-bone rig.

Run: blender --background --python scripts/build_ranger.py
"""
import bpy
import math
from pathlib import Path
from mathutils import Vector

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "public/assets/nachalo/characters/ranger/ranger_v2.glb"
OUT.parent.mkdir(parents=True, exist_ok=True)
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete(use_global=False)

def mat(name, color, metallic=0):
    m = bpy.data.materials.new(name)
    m.diffuse_color = (*color, 1)
    m.use_nodes = True
    bsdf = m.node_tree.nodes.get("Principled BSDF")
    bsdf.inputs["Base Color"].default_value = (*color, 1)
    bsdf.inputs["Metallic"].default_value = metallic
    bsdf.inputs["Roughness"].default_value = 0.82
    return m

cloth = mat("charcoal cloth", (0.095, 0.103, 0.112))
cape = mat("weathered cloak", (0.105, 0.115, 0.125))
leather = mat("dark brown leather", (0.20, 0.115, 0.077))
metal = mat("aged bronze", (0.32, 0.235, 0.17), 0.45)
edge = mat("light worn leather", (0.37, 0.255, 0.18))
inner = mat("muted teal lining", (0.025, 0.175, 0.20))
dark_metal = mat("dark forged steel", (0.15, 0.17, 0.19), 0.55)
skin = mat("warm skin", (0.48, 0.34, 0.25))
teal = mat("limited turquoise", (0.025, 0.39, 0.43), 0.25)
black = mat("face scarf", (0.045, 0.048, 0.05))

# The character is 1.68 Blender units tall; one game tile is one Three.js unit.
# Export scale is applied in the viewer, leaving clear floor contact at Z=0.
bpy.ops.object.armature_add(enter_editmode=True)
rig = bpy.context.object
rig.name = "Ranger_Armature"
rig.data.edit_bones.remove(rig.data.edit_bones[0])
def bone(name, head, tail, parent=None):
    b = rig.data.edit_bones.new(name)
    b.head, b.tail = head, tail
    if parent: b.parent = rig.data.edit_bones[parent]
    return b
bone("Root", (0,0,0), (0,0,0.12))
bone("Hips", (0,0,0.78), (0,0,0.92), "Root")
bone("Spine", (0,0,0.92), (0,0,1.27), "Hips")
bone("Chest", (0,0,1.27), (0,0,1.43), "Spine")
bone("Head", (0,0,1.43), (0,0,1.68), "Chest")
for side, sign in (("L", -1), ("R", 1)):
    bone(f"UpperArm.{side}", (sign*.25,0,1.38), (sign*.34,0,1.13), "Chest")
    bone(f"Forearm.{side}", (sign*.34,0,1.13), (sign*.37,0,0.89), f"UpperArm.{side}")
    bone(f"Hand.{side}", (sign*.37,0,.89), (sign*.37,0,.80), f"Forearm.{side}")
    bone(f"Thigh.{side}", (sign*.13,0,.81), (sign*.15,0,.46), "Hips")
    bone(f"Shin.{side}", (sign*.15,0,.46), (sign*.15,0,.13), f"Thigh.{side}")
    bone(f"Foot.{side}", (sign*.15,0,.13), (sign*.15,-.15,.075), f"Shin.{side}")
bpy.ops.object.mode_set(mode="OBJECT")

def bind(obj, name, material, joint):
    obj.name = name
    obj.data.materials.append(material)
    # Rigid weights are intentional for this low-poly prototype. Every visible
    # piece is skinned to the armature, including the separate bow mesh.
    group = obj.vertex_groups.new(name=joint)
    group.add(list(range(len(obj.data.vertices))), 1.0, "REPLACE")
    mod = obj.modifiers.new("Armature skin", "ARMATURE")
    mod.object = rig
    obj.parent = rig
    return obj

def uv(name, pos, scale, material, joint, segments=12):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=segments, ring_count=8, location=pos)
    o = bpy.context.object
    o.scale = scale
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    return bind(o, name, material, joint)

def cyl(name, pos, radius1, radius2, depth, material, joint, vertices=10):
    bpy.ops.mesh.primitive_cone_add(vertices=vertices, radius1=radius1, radius2=radius2,
                                    depth=depth, location=pos)
    return bind(bpy.context.object, name, material, joint)

def box(name, pos, scale, material, joint, angle=0):
    bpy.ops.mesh.primitive_cube_add(size=1, location=pos)
    o = bpy.context.object
    o.dimensions = scale
    o.rotation_euler[1] = angle
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    return bind(o, name, material, joint)

cyl("layered tunic", (0,0,1.11), .26,.22,.46, cloth,"Spine")
uv("leather chest", (0,-.07,1.26), (.245,.15,.22), leather,"Chest")
cyl("belt", (0,0,.89), .245,.245,.085, leather,"Hips")
box("bronze buckle", (0,-.253,.89), (.11,.025,.075), metal,"Hips")
box("diagonal chest strap", (.02,-.222,1.28), (.065,.025,.41), leather,"Chest", -.62)
uv("cape shoulders", (0,.115,1.34), (.34,.16,.18), cape,"Chest")
cyl("ragged cloak tail", (0,.16,.93), .34,.25,.78, cape,"Spine",12)
cyl("teal inner cloth", (0,-.18,.75), .11,.07,.36, teal,"Hips")
cyl("quiver", (.16,.25,1.19), .075,.067,.35, leather,"Chest",8)
for i in range(3):
    cyl(f"arrow shaft {i}", (.10+i*.055,.255,1.43+i*.015), .009,.009,.25, metal,"Chest",6)
    cyl(f"arrow fletching {i}", (.10+i*.055,.255,1.55+i*.015), .026,.007,.065, teal,"Chest",5)
uv("hood crown", (0,0,1.56), (.24,.23,.21), cape,"Head")
cyl("hood point", (0,.035,1.69), .21,.015,.20, cape,"Head",8)
uv("shadowed face", (0,-.165,1.54), (.145,.085,.11), skin,"Head")
box("face scarf", (0,-.215,1.495), (.24,.04,.085), black,"Head")
for side, sign in (("L",-1),("R",1)):
    uv(f"eye {side}", (sign*.065,-.239,1.565), (.021,.01,.014), teal,"Head",8)
    uv(f"sleeve {side}", (sign*.29,0,1.23), (.105,.105,.185), cloth,f"UpperArm.{side}")
    cyl(f"bracer {side}", (sign*.355,0,1.02), .09,.08,.21, metal,f"Forearm.{side}")
    uv(f"glove {side}", (sign*.37,0,.84), (.075,.075,.095), leather,f"Hand.{side}")
    cyl(f"trouser {side}", (sign*.14,0,.63), .13,.11,.37, cloth,f"Thigh.{side}")
    cyl(f"shin guard {side}", (sign*.15,-.02,.31), .11,.10,.31, leather,f"Shin.{side}")
    box(f"boot {side}", (sign*.15,-.075,.085), (.21,.32,.16), leather,f"Foot.{side}")
    box(f"knee armor {side}", (sign*.15,-.11,.46), (.2,.05,.14), metal,f"Shin.{side}")

# RANGER V2 DETAIL PASS ------------------------------------------------------
# All components are mesh objects bound to the original skeleton. Additional
# objects do not introduce joints or change animation names. They enhance the
# silhouette without disturbing the current live-match animation integration.

def bevel_box(name, pos, dimensions, material, joint, bevel=.025, angle=0):
    bpy.ops.mesh.primitive_cube_add(size=1, location=pos)
    obj = bpy.context.object
    obj.dimensions = dimensions
    obj.rotation_euler[1] = angle
    bpy.ops.object.transform_apply(location=False, rotation=False, scale=True)
    mod = obj.modifiers.new('softened edges', 'BEVEL')
    mod.width = bevel
    mod.segments = 2
    bpy.context.view_layer.objects.active = obj
    bpy.ops.object.modifier_apply(modifier=mod.name)
    nmod = obj.modifiers.new('weighted normals', 'WEIGHTED_NORMAL')
    bpy.ops.object.modifier_apply(modifier=nmod.name)
    return bind(obj, name, material, joint)

def tube_between(name, a, b, radius, material, joint, vertices=10):
    a, b = Vector(a), Vector(b)
    delta = b-a
    middle = (a+b)*.5
    bpy.ops.mesh.primitive_cylinder_add(vertices=vertices, radius=radius,
                                        depth=delta.length, location=middle)
    obj = bpy.context.object
    obj.rotation_euler = delta.to_track_quat('Z', 'Y').to_euler()
    return bind(obj, name, material, joint)

# Reduce the crude silhouette of the original torso by adding overlapping
# shaped layers, armour panels and narrow leather hems.
uv('padded chest underlayer', (0,-.04,1.215), (.255,.142,.228), cloth, 'Chest', 20)
for sign, side in ((-1,'L'), (1,'R')):
    bevel_box('leather rib armour '+side, (sign*.135,-.160,1.215),
              (.195,.045,.25), leather,'Chest', .03,sign*.09)
    bevel_box('shoulder leather cap '+side, (sign*.265,-.012,1.355),
              (.23,.19,.095),leather,'UpperArm.'+side,.045)
    bevel_box('bracer plating '+side,(sign*.359,-.095,1.035),
              (.151,.045,.22),dark_metal,'Forearm.'+side,.024)
    bevel_box('bracer teal jewel '+side,(sign*.356,-.123,1.055),
              (.042,.016,.073),teal,'Forearm.'+side,.01)
    bevel_box('hand back glove '+side,(sign*.37,-.067,.87),
              (.142,.07,.14),leather,'Hand.'+side,.025)
    bevel_box('knee tapered plate '+side,(sign*.15,-.145,.463),
              (.19,.055,.177),dark_metal,'Shin.'+side,.033)
    bevel_box('boot instep '+side,(sign*.15,-.155,.122),
              (.195,.28,.118),leather,'Foot.'+side,.039)
    bevel_box('boot toe '+side,(sign*.15,-.225,.065),
              (.19,.16,.095),dark_metal,'Foot.'+side,.028)
    tube_between('boot top piping '+side,
                 (sign*.15,-.09,.25),(sign*.15,.07,.25),.018,edge,'Shin.'+side)

# Hood: the cloth brim is several separate overlapping segments around the
# shadowed face rather than a single bald sphere. The angular rear crown and
# cheek folds keep the characteristic ranger silhouette at game camera scale.
uv('hood padded outer shell', (0,.058,1.572), (.269,.227,.242),cape,'Head',20)
uv('face cavity shadow', (0,-.232,1.557), (.156,.023,.135),black,'Head',16)
for side, sign in (('L',-1),('R',1)):
    tube_between('hood opening side '+side,
                 (sign*.164,-.246,1.43),(sign*.171,-.239,1.68),
                 .037,cape,'Head',10)
    tube_between('hood side seam '+side,
                 (sign*.245,-.027,1.70),(sign*.236,.1,1.49),
                 .011,edge,'Head',8)
tube_between('hood forehead roll',(-.164,-.242,1.675),(.164,-.242,1.675),
             .030,cape,'Head',10)
bevel_box('face cloth mask',(0,-.245,1.49),(.29,.047,.11),black,'Head',.037)

# Asymmetric segmented cloak tails. These panels hang behind the legs and
# change with the hips/chest; they are deliberately short to avoid clipping
# through the floor and do not require cloth simulation.
for idx,(x,ztop,bottom,width) in enumerate(((-.24,1.31,.77,.18),(0,1.33,.64,.21),(.23,1.30,.79,.175))):
    tube_between('cloak structural fold '+str(idx), (x,.245,ztop),
                 (x*1.19,.27,bottom),.033,cape,'Spine',8)
    bevel_box('cloak fabric panel '+str(idx),(x*1.07,.252,(ztop+bottom)/2),
              (width,.034,ztop-bottom),cape,'Spine',.032, -.04 if idx==0 else .04)
    bevel_box('cloak inner blue hem '+str(idx),(x*1.07,.225,bottom+.034),
              (width*.78,.025,.076),inner,'Spine',.013)

# Functional equipment: visible cross-body harness, pouch, cloak clasp,
# quiver rim, and layered cloth tabard.
tube_between('cross body strap seam',(-.185,-.229,1.36),(.17,-.235,1.04),
             .018,edge,'Chest')
bevel_box('belt side pouch',(.24,-.114,.895),(.15,.145,.15),leather,'Hips',.028)
bevel_box('pouch flap',(.24,-.198,.949),(.16,.022,.065),edge,'Hips',.014)
uv('cloak clasp metallic',(-.19,-.164,1.418),(.046,.027,.050),metal,'Chest',12)
uv('cloak clasp teal gem',(-.19,-.191,1.421),(.023,.013,.029),teal,'Chest',12)
bevel_box('tabard folds',(0,-.201,.77),(.205,.041,.395),inner,'Hips',.028)
tube_between('quiver leather trim',(.17,.25,1.34),(.17,.25,1.06),
             .018,edge,'Chest')

# A stronger bow silhouette: a dark wooden grip and visible narrow tips.
# This geometry is distinct from the original skinned Bow object.
bevel_box('bow grip leather',(-.432,-.106,.865),(.07,.07,.21),leather,'Hand.L',.024)
bevel_box('bow tip upper',(-.398,-.106,1.25),(.044,.050,.095),dark_metal,'Hand.L',.012)
bevel_box('bow tip lower',(-.398,-.106,.52),(.044,.050,.095),dark_metal,'Hand.L',.012)
# ---------------------------------------------------------------------------

# Bow is a distinct skinned object in the left hand. Curved stave and string
# are joined into one mesh before binding, so it remains a single attachment.
bow_parts=[]
for i in range(13):
    z0 = .53 + i*.055
    x0 = -.39 - .12 * math.sin(i/12*math.pi)
    bpy.ops.mesh.primitive_uv_sphere_add(segments=8, ring_count=4, location=(x0,-.10,z0))
    o=bpy.context.object; o.scale=(.025,.025,.066); bow_parts.append(o)
for i in range(11):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=6, ring_count=4, location=(-.39,-.10,.56+i*.063))
    o=bpy.context.object; o.scale=(.005,.005,.034); bow_parts.append(o)
bpy.ops.object.select_all(action="DESELECT")
for o in bow_parts: o.select_set(True)
bpy.context.view_layer.objects.active=bow_parts[0]
bpy.ops.object.convert(target="MESH")
bpy.ops.object.join()
bind(bpy.context.object,"Bow",metal,"Hand.L")

def action(name, frames, keys):
    act=bpy.data.actions.new(name)
    rig.animation_data_create()
    rig.animation_data.action=act
    for frame, rotations in keys:
        for joint, axes in rotations.items():
            pb=rig.pose.bones[joint]
            pb.rotation_mode="XYZ"
            pb.rotation_euler=axes
            pb.keyframe_insert(data_path="rotation_euler",frame=frame,group=joint)
    for fc in act.fcurves:
        for kp in fc.keyframe_points: kp.interpolation="BEZIER"
    if name in ("Idle","Walk"):
        for fc in act.fcurves:
            fc.modifiers.new(type="CYCLES")
    act.use_fake_user=True
    return act

Z=(0,0,0)
action("Idle",49,[(1,{"Chest":Z,"Head":Z,"UpperArm.L":Z,"UpperArm.R":Z}),
    (25,{"Chest":(.018,0,0),"Head":(-.016,0,0),"UpperArm.L":(-.025,0,0),"UpperArm.R":(-.025,0,0)}),
    (49,{"Chest":Z,"Head":Z,"UpperArm.L":Z,"UpperArm.R":Z})])
walk=[]
for frame,phase in ((1,0),(9,math.pi/2),(17,math.pi),(25,3*math.pi/2),(33,2*math.pi)):
    a=math.sin(phase)*.48
    walk.append((frame,{"Thigh.L":(a,0,0),"Thigh.R":(-a,0,0),
      "Shin.L":(max(0,-a)*.75,0,0),"Shin.R":(max(0,a)*.75,0,0),
      "UpperArm.L":(-a*.65,0,0),"UpperArm.R":(a*.65,0,0),
      "Chest":(0,0,math.sin(phase)*.025)}))
action("Walk",33,walk)
action("Attack_Bow",38,[(1,{"Chest":Z,"UpperArm.L":Z,"UpperArm.R":Z,"Forearm.R":Z}),
 (10,{"Chest":(0,0,-.18),"UpperArm.L":(-.95,0,-.22),"UpperArm.R":(-.9,0,.7),"Forearm.R":(-.65,0,0)}),
 (24,{"Chest":(0,0,-.18),"UpperArm.L":(-1.1,0,-.22),"UpperArm.R":(-1.05,0,.9),"Forearm.R":(-1.0,0,0)}),
 (27,{"Chest":(0,0,-.1),"UpperArm.L":(-1.1,0,-.22),"UpperArm.R":(-.4,0,.5),"Forearm.R":(-.2,0,0)}),
 (38,{"Chest":Z,"UpperArm.L":Z,"UpperArm.R":Z,"Forearm.R":Z})])
action("Hit",22,[(1,{"Spine":Z,"Head":Z}),(5,{"Spine":(.26,0,.18),"Head":(-.18,0,0)}),
 (12,{"Spine":(-.08,0,-.08),"Head":(.06,0,0)}),(22,{"Spine":Z,"Head":Z})])
action("Death",48,[(1,{"Root":Z,"Spine":Z,"Thigh.L":Z,"Thigh.R":Z}),
 (18,{"Root":(.65,0,.3),"Spine":(.35,0,0),"Thigh.L":(-.4,0,0),"Thigh.R":(.35,0,0)}),
 (48,{"Root":(1.25,0,.3),"Spine":(.45,0,0),"Thigh.L":(-.6,0,0),"Thigh.R":(.5,0,0)})])
rig.animation_data.action=None
bpy.ops.object.select_all(action="DESELECT")
rig.select_set(True)
for obj in bpy.data.objects:
    if obj.type=="MESH": obj.select_set(True)
bpy.context.view_layer.objects.active=rig
bpy.ops.export_scene.gltf(filepath=str(OUT), export_format="GLB", use_selection=True,
    export_animations=True, export_animation_mode="ACTIONS")
print(f"Exported {OUT} ({OUT.stat().st_size} bytes)")
