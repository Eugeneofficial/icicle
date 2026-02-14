#!/usr/bin/env python3
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FRONT = ROOT / "cmd" / "icicle-wails" / "frontend" / "index.html"


def extract_lang_obj(src: str, lang: str) -> str:
    m = re.search(rf"{lang}\s*:\s*\{{", src)
    if not m:
        raise RuntimeError(f"language block '{lang}' not found")
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
    text = "".join(out)
    if depth != 0:
        raise RuntimeError(f"failed to parse language block '{lang}'")
    return text


def keys_of(obj_text: str):
    keys = []
    i = 0
    depth = 0
    in_str = False
    quote = ""
    esc = False
    while i < len(obj_text):
        ch = obj_text[i]
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == quote:
                in_str = False
            i += 1
            continue
        if ch in ('"', "'"):
            in_str = True
            quote = ch
            i += 1
            continue
        if ch == "{":
            depth += 1
            i += 1
            continue
        if ch == "}":
            depth -= 1
            i += 1
            continue
        if depth == 1:
            if ch.isalpha() or ch == "_":
                j = i + 1
                while j < len(obj_text) and (obj_text[j].isalnum() or obj_text[j] == "_"):
                    j += 1
                ident = obj_text[i:j]
                k = j
                while k < len(obj_text) and obj_text[k].isspace():
                    k += 1
                if k < len(obj_text) and obj_text[k] == ":":
                    keys.append(ident)
                    i = k + 1
                    continue
        i += 1
    return set(keys)


def main() -> int:
    src = FRONT.read_text(encoding="utf-8")
    en = extract_lang_obj(src, "en")
    ru = extract_lang_obj(src, "ru")
    en_keys = keys_of(en)
    ru_keys = keys_of(ru)

    missing_in_ru = sorted(en_keys - ru_keys)
    extra_in_ru = sorted(ru_keys - en_keys)

    if missing_in_ru or extra_in_ru:
        print("Glossary QA failed")
        if missing_in_ru:
            print("Missing in RU:", ", ".join(missing_in_ru))
        if extra_in_ru:
            print("Extra in RU:", ", ".join(extra_in_ru))
        return 1

    print(f"Glossary QA OK: {len(en_keys)} keys")
    return 0


if __name__ == "__main__":
    sys.exit(main())
