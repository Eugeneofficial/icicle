#!/usr/bin/env python3
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FRONT = ROOT / "cmd" / "icicle-wails" / "frontend" / "index.html"


def extract_lang(src: str, lang: str) -> str:
    m = re.search(rf"{lang}\s*:\s*\{{", src)
    if not m:
        raise RuntimeError(f"lang block not found: {lang}")
    i = m.end() - 1
    depth = 0
    in_str = False
    quote = ""
    esc = False
    out = []
    while i < len(src):
        ch = src[i]
        out.append(ch)
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == quote:
                in_str = False
        else:
            if ch in ('"', "'"):
                in_str = True
                quote = ch
            elif ch == "{":
                depth += 1
            elif ch == "}":
                depth -= 1
                if depth == 0:
                    break
        i += 1
    return "".join(out)


def keys(obj: str):
    return set(re.findall(r"\b([A-Za-z0-9_]+)\s*:", obj))


def main() -> int:
    src = FRONT.read_text(encoding="utf-8")
    en = keys(extract_lang(src, "en"))
    ru = keys(extract_lang(src, "ru"))
    used = set(re.findall(r"tr\('([A-Za-z0-9_]+)'\)", src))

    miss_en = sorted(used - en)
    miss_ru = sorted(used - ru)
    if miss_en or miss_ru:
        print("Localization regression failed")
        if miss_en:
            print("Missing in EN:", ", ".join(miss_en))
        if miss_ru:
            print("Missing in RU:", ", ".join(miss_ru))
        return 1

    print(f"Localization regression OK: used={len(used)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
