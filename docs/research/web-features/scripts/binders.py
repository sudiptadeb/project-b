"""List the interfaces a binder function adds, with the #if conditions around each.

Usage: binders.py <file> <function-name>
Prints: line  interface  conditions
"""
import re
import sys

path, func = sys.argv[1], sys.argv[2]
lines = open(path).read().split("\n")
start = next(i for i, l in enumerate(lines) if re.match(r"^\w.*\b%s\(" % re.escape(func), l))
depth = 0
stack = []
began = False
buf = ""
for i in range(start, len(lines)):
    l = lines[i]
    s = l.strip()
    if s.startswith("#if"):
        stack.append(s[3:].strip())
    elif s.startswith("#elif"):
        stack[-1] = "!(%s) && %s" % (stack[-1], s[5:].strip())
    elif s.startswith("#else"):
        stack[-1] = "!(%s)" % stack[-1]
    elif s.startswith("#endif"):
        stack.pop()
    depth += l.count("{") - l.count("}")
    if "{" in l:
        began = True
    buf += " " + s
    for m in re.finditer(r"(?:map|registry)->Add(?:Interface)?<\s*([\w:]+)\s*>\s*\(\s*&?([\w:<>]*)", buf):
        empty = "EMPTY" if "EmptyBinder" in m.group(2) or "DoNothing" in m.group(2) else ""
        print("%5d  %-60s %-6s %s" % (i + 1, m.group(1), empty, " && ".join(stack)))
    if ";" in s or s.startswith("#") or s == "":
        buf = ""
    if began and depth == 0:
        break
