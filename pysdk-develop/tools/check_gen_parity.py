"""对账脚本：Go gen/idl_gen.go 静态数据 ↔ Python 运行时 IDL 加载。

用法:
    python tools/check_gen_parity.py [gosdk-develop 路径]

断言：app 名集合、每 app instruction 名集合、每 instruction 判别器数值、
自定义类型名与 typeTag 全部一致。差异清单打印，非零退出码表示不对账。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path


def _flat_fields(text: str, start: int):
    """读取 start 处 '{' 的**顶层** `Key: value` 字段（跳过嵌套块）。"""
    fields = {}
    depth = 1
    i = text.index("{", start) + 1
    line_start = i
    while i < len(text) and depth > 0:
        ch = text[i]
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
        elif ch == "\n" and depth == 1:
            line = text[line_start:i]
            m = re.match(r"\s*(\w+):\s*(.+?),?\s*$", line)
            if m and "{" not in m.group(2) and "[" not in m.group(2):
                fields[m.group(1)] = m.group(2).strip().strip(",").strip()
            line_start = i + 1
        i += 1
    return fields


def parse_go_idl_gen(path: Path):
    """从 Go idl_gen.go 提取 (app, instruction, discriminator) 与 (app, type, typeTag)。"""
    text = path.read_text(encoding="utf-8")

    instructions = {}
    types = {}
    current_app = None

    for m in re.finditer(r"provider\.(IDL|Instruction|IDLType)\{", text):
        kind = m.group(1)
        fields = _flat_fields(text, m.end() - 1)
        if kind == "IDL":
            name_m = re.search(r'Name:\s*"([^"]+)"', text[m.end(): m.end() + 200])
            # Metadata 块内的 Name（app 名）
            app_m = re.search(
                r"Metadata:\s*provider\.Metadata\{\s*Name:\s*\"([^\"]+)\"",
                text[m.end(): m.end() + 400],
            )
            current_app = (app_m or name_m).group(1)
            instructions.setdefault(current_app, {})
            types.setdefault(current_app, {})
        elif kind == "Instruction" and current_app:
            name = fields.get("Name", "").strip('"')
            disc = fields.get("Discriminator")
            if name and disc is not None:
                instructions[current_app][name] = int(disc)
        elif kind == "IDLType" and current_app:
            name = fields.get("Name", "").strip('"')
            tag = fields.get("TypeTag")
            if name and tag is not None:
                types[current_app][name] = int(tag)

    return instructions, types


def main() -> int:
    gosdk = Path(sys.argv[1] if len(sys.argv) > 1 else "gosdk-develop")
    go_file = gosdk / "gen" / "idl_gen.go"
    if not go_file.exists():
        print(f"ERROR: {go_file} not found")
        return 1

    sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))
    from milon_sdk import gen

    go_instructions, go_types = parse_go_idl_gen(go_file)
    py_idls = gen.default_idls()

    py_instructions = {}
    py_types = {}
    for name, idl in py_idls.items():
        py_instructions[name] = {i.Name: i.Discriminator for i in idl.Instructions}
        py_types[name] = {t.Name: t.TypeTag for t in idl.Types}

    problems = []

    go_apps, py_apps = set(go_instructions), set(py_instructions)
    if go_apps != py_apps:
        problems.append(
            f"app 集合不一致: go-only={sorted(go_apps - py_apps)} "
            f"py-only={sorted(py_apps - go_apps)}"
        )

    for app in sorted(go_apps & py_apps):
        gi, pi = go_instructions[app], py_instructions[app]
        if set(gi) != set(pi):
            problems.append(
                f"[{app}] instruction 名不一致: "
                f"go-only={sorted(set(gi) - set(pi))} "
                f"py-only={sorted(set(pi) - set(gi))}"
            )
        for name in sorted(set(gi) & set(pi)):
            if gi[name] != pi[name]:
                problems.append(
                    f"[{app}.{name}] 判别器不一致: go={gi[name]} py={pi[name]}"
                )

        gt, pt = go_types.get(app, {}), py_types.get(app, {})
        if set(gt) != set(pt):
            problems.append(
                f"[{app}] 类型名不一致: go-only={sorted(set(gt) - set(pt))} "
                f"py-only={sorted(set(pt) - set(gt))}"
            )
        for name in sorted(set(gt) & set(pt)):
            if gt[name] != pt[name]:
                problems.append(
                    f"[{app}.{name}] typeTag 不一致: go={gt[name]} py={pt[name]}"
                )

    total_ix = sum(len(v) for v in go_instructions.values())
    print(
        f"对账完成: {len(go_apps)} apps / {total_ix} instructions / "
        f"{sum(len(v) for v in go_types.values())} types"
    )

    if problems:
        print(f"\n发现 {len(problems)} 处差异:")
        for p in problems:
            print("  -", p)
        return 1
    print("PASS: Go ↔ Python IDL 数据完全一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())
