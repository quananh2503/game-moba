import re

with open('/home/quananh/game/game-server/logic.go', 'r') as f:
    orig = f.read()

# Pattern for Collider: addComponent(engine, E_NAME, Collider{ShapeType: TYPE, ... })
def repl(m):
    indent = m.group(1)
    engine = m.group(2)
    ent = m.group(3)
    coll_args = m.group(4)
    # Extract ShapeType
    shape_match = re.search(r'ShapeType:\s*([^,}]+)', coll_args)
    shape = shape_match.group(1) if shape_match else "def.ShapeCircle"
    # Extract Radius
    rad_match = re.search(r'Radius:\s*([^,}]+)', coll_args)
    rad = rad_match.group(1) if rad_match else "0"
    # Extract Width
    wid_match = re.search(r'Width:\s*([^,}]+)', coll_args)
    wid = wid_match.group(1) if wid_match else "0"
    # Extract Height
    hgt_match = re.search(r'Height:\s*([^,}]+)', coll_args)
    hgt = hgt_match.group(1) if hgt_match else "0"
    
    # We need to find the angle. The angle is usually in the parameter of the enclosing function, or we can just use 0 if it's a circle.
    # It might be in the transform: addComponent(engine, e, Transform{... Angle: angle })
    # For now, let's just cheat and assume angle is available as `angle`, except where we have explicit 0 or if shape is circle where angle doesn't matter.
    angle = "0"
    if 'def.ShapeOBB' in shape:
        angle = "angle"

    # For safety, if it's 1 it means def.ShapeCircle.
    if shape == "1": shape = "def.ShapeCircle"

    aabb_line = f"{indent}addComponent({engine}, {ent}, CalculateAABB({shape}, {rad}, {wid}, {hgt}, {angle}))"
    return m.group(0) + "\n" + aabb_line

new_content = re.sub(r'^(\s*)addComponent\(([^,]+),\s*([^,]+),\s*Collider\{(.*?)\}\).*?$', repl, orig, flags=re.MULTILINE)

with open('/home/quananh/game/game-server/logic.go', 'w') as f:
    f.write(new_content)

print("Done")
