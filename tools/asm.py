#!/usr/bin/env python3
"""Tiny 6502 assembler: labels, .org, .byte, .word, standard mnemonics."""
from __future__ import annotations

import re
import sys
from pathlib import Path

IMM, ZP, ZPX, ZPY, ABS, ABSX, ABSY, IND, INDX, INDY, REL, IMP, ACC = range(13)

OPCODES: dict[tuple[str, int], int] = {}


def op(mne: str, mode: int, code: int) -> None:
    OPCODES[(mne.upper(), mode)] = code


def load_ops() -> None:
    for m, imm, zp, zpx, abs_, absx, absy, indx, indy in [
        ("ORA", 0x09, 0x05, 0x15, 0x0D, 0x1D, 0x19, 0x01, 0x11),
        ("AND", 0x29, 0x25, 0x35, 0x2D, 0x3D, 0x39, 0x21, 0x31),
        ("EOR", 0x49, 0x45, 0x55, 0x4D, 0x5D, 0x59, 0x41, 0x51),
        ("ADC", 0x69, 0x65, 0x75, 0x6D, 0x7D, 0x79, 0x61, 0x71),
        ("STA", None, 0x85, 0x95, 0x8D, 0x9D, 0x99, 0x81, 0x91),
        ("LDA", 0xA9, 0xA5, 0xB5, 0xAD, 0xBD, 0xB9, 0xA1, 0xB1),
        ("CMP", 0xC9, 0xC5, 0xD5, 0xCD, 0xDD, 0xD9, 0xC1, 0xD1),
        ("SBC", 0xE9, 0xE5, 0xF5, 0xED, 0xFD, 0xF9, 0xE1, 0xF1),
    ]:
        if imm is not None:
            op(m, IMM, imm)
        op(m, ZP, zp)
        op(m, ZPX, zpx)
        op(m, ABS, abs_)
        op(m, ABSX, absx)
        op(m, ABSY, absy)
        op(m, INDX, indx)
        op(m, INDY, indy)

    for m, imm, zp, zpy, abs_, absy in [
        ("LDX", 0xA2, 0xA6, 0xB6, 0xAE, 0xBE),
        ("STX", None, 0x86, 0x96, 0x8E, None),
    ]:
        if imm is not None:
            op(m, IMM, imm)
        op(m, ZP, zp)
        op(m, ZPY, zpy)
        op(m, ABS, abs_)
        if absy is not None:
            op(m, ABSY, absy)

    for m, imm, zp, zpx, abs_, absx in [
        ("LDY", 0xA0, 0xA4, 0xB4, 0xAC, 0xBC),
        ("STY", None, 0x84, 0x94, 0x8C, None),
    ]:
        if imm is not None:
            op(m, IMM, imm)
        op(m, ZP, zp)
        op(m, ZPX, zpx)
        op(m, ABS, abs_)
        if absx is not None:
            op(m, ABSX, absx)

    for m, acc, zp, zpx, abs_, absx in [
        ("ASL", 0x0A, 0x06, 0x16, 0x0E, 0x1E),
        ("LSR", 0x4A, 0x46, 0x56, 0x4E, 0x5E),
        ("ROL", 0x2A, 0x26, 0x36, 0x2E, 0x3E),
        ("ROR", 0x6A, 0x66, 0x76, 0x6E, 0x7E),
    ]:
        op(m, ACC, acc)
        op(m, IMP, acc)
        op(m, ZP, zp)
        op(m, ZPX, zpx)
        op(m, ABS, abs_)
        op(m, ABSX, absx)

    for m, zp, zpx, abs_, absx in [
        ("DEC", 0xC6, 0xD6, 0xCE, 0xDE),
        ("INC", 0xE6, 0xF6, 0xEE, 0xFE),
    ]:
        op(m, ZP, zp)
        op(m, ZPX, zpx)
        op(m, ABS, abs_)
        op(m, ABSX, absx)

    op("BIT", ZP, 0x24)
    op("BIT", ABS, 0x2C)
    op("JMP", ABS, 0x4C)
    op("JMP", IND, 0x6C)
    op("JSR", ABS, 0x20)
    op("CPX", IMM, 0xE0)
    op("CPX", ZP, 0xE4)
    op("CPX", ABS, 0xEC)
    op("CPY", IMM, 0xC0)
    op("CPY", ZP, 0xC4)
    op("CPY", ABS, 0xCC)

    for m, code in [
        ("BRK", 0x00), ("PHP", 0x08), ("CLC", 0x18), ("PLP", 0x28),
        ("SEC", 0x38), ("RTI", 0x40), ("PHA", 0x48), ("CLI", 0x58),
        ("RTS", 0x60), ("PLA", 0x68), ("SEI", 0x78), ("DEY", 0x88),
        ("TXA", 0x8A), ("TYA", 0x98), ("TXS", 0x9A), ("TAY", 0xA8),
        ("TAX", 0xAA), ("CLV", 0xB8), ("TSX", 0xBA), ("INY", 0xC8),
        ("DEX", 0xCA), ("CLD", 0xD8), ("INX", 0xE8), ("NOP", 0xEA),
        ("SED", 0xF8),
    ]:
        op(m, IMP, code)

    for m, code in [
        ("BPL", 0x10), ("BMI", 0x30), ("BVC", 0x50), ("BVS", 0x70),
        ("BCC", 0x90), ("BCS", 0xB0), ("BNE", 0xD0), ("BEQ", 0xF0),
    ]:
        op(m, REL, code)


load_ops()

HEX = re.compile(r"^\$([0-9A-Fa-f]+)$")
DEC = re.compile(r"^(\d+)$")


def parse_num(s: str, labels: dict[str, int], pc: int) -> int:
    s = s.strip()
    if s.upper() == "*":
        return pc
    if s.startswith("<"):
        return parse_num(s[1:], labels, pc) & 0xFF
    if s.startswith(">"):
        return (parse_num(s[1:], labels, pc) >> 8) & 0xFF
    m = HEX.match(s)
    if m:
        return int(m.group(1), 16)
    m = DEC.match(s)
    if m:
        return int(m.group(1), 10)
    if s in labels:
        return labels[s]
    raise KeyError(s)


def is_force_zp(expr: str) -> bool:
    s = expr.strip()
    m = HEX.match(s)
    if m:
        return int(m.group(1), 16) <= 0xFF
    m = DEC.match(s)
    if m:
        return int(m.group(1), 10) <= 255
    return False


def parse_bytes(arg: str, labels: dict[str, int], pc: int) -> list[int]:
    out: list[int] = []
    i = 0
    while i < len(arg):
        if arg[i] in " \t,":
            i += 1
            continue
        if arg[i] in "\"'":
            q = arg[i]
            i += 1
            while i < len(arg) and arg[i] != q:
                out.append(ord(arg[i]))
                i += 1
            i += 1
            continue
        j = i
        while j < len(arg) and arg[j] not in ", \t":
            j += 1
        out.append(parse_num(arg[i:j], labels, pc) & 0xFF)
        i = j
    return out


def classify(operand: str) -> tuple[int, str]:
    o = operand.strip()
    if o == "" or o.upper() == "A":
        return IMP if o == "" else ACC, ""
    if o.startswith("#"):
        return IMM, o[1:]
    ou = o.upper()
    if ou.startswith("(") and ou.endswith(",X)"):
        return INDX, o[1:-3]
    if ou.startswith("(") and ou.endswith("),Y"):
        return INDY, o[1:-3]
    if ou.startswith("(") and ou.endswith(")"):
        return IND, o[1:-1]
    if ou.endswith(",X"):
        return "ABSX_OR_ZPX", o[:-2]
    if ou.endswith(",Y"):
        return "ABSY_OR_ZPY", o[:-2]
    return "ABS_OR_ZP", o


class Line:
    __slots__ = ("label", "mne", "operand", "raw")

    def __init__(self, label: str | None, mne: str | None, operand: str, raw: str):
        self.label = label
        self.mne = mne
        self.operand = operand
        self.raw = raw


def parse_line(raw: str) -> Line | None:
    s = raw.split(";", 1)[0].rstrip()
    if not s.strip():
        return None
    stripped = s.strip()
    if "=" in stripped and ":" not in stripped.split("=", 1)[0]:
        name, expr = stripped.split("=", 1)
        name = name.strip()
        if name and " " not in name:
            return Line(name, ".EQU", expr.strip(), raw)
    label = None
    if ":" in s:
        before, after = s.split(":", 1)
        if before.strip() and not before.strip()[0].isspace() and " " not in before.strip():
            label = before.strip()
            s = after
    s = s.strip()
    if not s:
        return Line(label, None, "", raw)
    parts = s.split(None, 1)
    return Line(label, parts[0].upper(), parts[1] if len(parts) > 1 else "", raw)


def assemble(src: str, origin: int = 0x0300) -> tuple[int, bytes, dict[str, int]]:
    lines = [parse_line(x) for x in src.splitlines()]
    lines = [ln for ln in lines if ln]
    labels: dict[str, int] = {}
    pc = origin

    def size_of(ln: Line) -> int:
        if not ln.mne:
            return 0
        if ln.mne in (".ORG", ".EQU"):
            return 0
        if ln.mne in (".BYTE", ".DB"):
            try:
                return len(parse_bytes(ln.operand, labels, pc))
            except KeyError:
                return len([t for t in re.split(r"[,\s]+", ln.operand) if t])
        if ln.mne in (".WORD", ".DW"):
            return 2 * len([t for t in re.split(r"[,\s]+", ln.operand) if t])
        if ln.mne in (".RES",):
            return parse_num(ln.operand, labels, pc)
        if (ln.mne, REL) in OPCODES:
            return 2
        mode, _ = classify(ln.operand)
        if mode in (IMP, ACC):
            return 1
        if mode == REL:
            return 2
        if mode == IMM:
            return 2
        if mode in (INDX, INDY):
            return 2
        if mode == IND:
            return 3
        if mode in ("ABSX_OR_ZPX", "ABSY_OR_ZPY", "ABS_OR_ZP"):
            _, expr = classify(ln.operand)
            return 2 if is_force_zp(expr) else 3
        return 3

    # pass 1
    pc = origin
    for ln in lines:
        if ln.mne == ".EQU":
            labels[ln.label] = parse_num(ln.operand, labels, pc)
            continue
        if ln.label:
            labels[ln.label] = pc
        if ln.mne == ".ORG":
            pc = parse_num(ln.operand, labels, pc)
            if ln.label:
                labels[ln.label] = pc
            continue
        pc += size_of(ln)

    # pass 2
    pc = origin
    start = origin
    blob = bytearray()
    min_addr = origin
    max_addr = origin
    image = bytearray(65536)
    used_lo, used_hi = 65535, 0

    def emit(addr: int, data: list[int]) -> None:
        nonlocal used_lo, used_hi
        for i, b in enumerate(data):
            image[(addr + i) & 0xFFFF] = b & 0xFF
            a = (addr + i) & 0xFFFF
            used_lo = min(used_lo, a)
            used_hi = max(used_hi, a)

    pc = origin
    for ln in lines:
        if ln.mne == ".ORG":
            pc = parse_num(ln.operand, labels, pc)
            continue
        if ln.mne == ".EQU":
            continue
        if not ln.mne:
            continue
        if ln.mne in (".BYTE", ".DB"):
            data = parse_bytes(ln.operand, labels, pc)
            emit(pc, data)
            pc += len(data)
            continue
        if ln.mne in (".WORD", ".DW"):
            parts = [t for t in re.split(r"[,\s]+", ln.operand) if t]
            data = []
            for p in parts:
                w = parse_num(p, labels, pc)
                data += [w & 0xFF, (w >> 8) & 0xFF]
            emit(pc, data)
            pc += len(data)
            continue
        if ln.mne == ".RES":
            n = parse_num(ln.operand, labels, pc)
            emit(pc, [0] * n)
            pc += n
            continue

        if (ln.mne, REL) in OPCODES:
            tgt = parse_num(ln.operand, labels, pc)
            off = tgt - (pc + 2)
            if off < -128 or off > 127:
                raise ValueError(f"branch out of range at ${pc:04X} -> ${tgt:04X} ({ln.raw})")
            emit(pc, [OPCODES[(ln.mne, REL)], off & 0xFF])
            pc += 2
            continue

        mode_hint, expr = classify(ln.operand)
        if mode_hint in (IMP, ACC):
            key_mode = IMP
            if (ln.mne, ACC) in OPCODES and (ln.operand.strip() == "" or ln.operand.strip().upper() == "A"):
                key_mode = ACC
            if (ln.mne, key_mode) not in OPCODES:
                key_mode = IMP
            emit(pc, [OPCODES[(ln.mne, key_mode)]])
            pc += 1
            continue
        if mode_hint == IMM:
            emit(pc, [OPCODES[(ln.mne, IMM)], parse_num(expr, labels, pc) & 0xFF])
            pc += 2
            continue
        if mode_hint == INDX:
            emit(pc, [OPCODES[(ln.mne, INDX)], parse_num(expr, labels, pc) & 0xFF])
            pc += 2
            continue
        if mode_hint == INDY:
            emit(pc, [OPCODES[(ln.mne, INDY)], parse_num(expr, labels, pc) & 0xFF])
            pc += 2
            continue
        if mode_hint == IND:
            w = parse_num(expr, labels, pc)
            emit(pc, [OPCODES[(ln.mne, IND)], w & 0xFF, (w >> 8) & 0xFF])
            pc += 3
            continue

        val = parse_num(expr, labels, pc)
        if mode_hint == "ABSX_OR_ZPX":
            if is_force_zp(expr) and (ln.mne, ZPX) in OPCODES:
                emit(pc, [OPCODES[(ln.mne, ZPX)], val & 0xFF])
                pc += 2
            else:
                emit(pc, [OPCODES[(ln.mne, ABSX)], val & 0xFF, (val >> 8) & 0xFF])
                pc += 3
        elif mode_hint == "ABSY_OR_ZPY":
            if is_force_zp(expr) and (ln.mne, ZPY) in OPCODES:
                emit(pc, [OPCODES[(ln.mne, ZPY)], val & 0xFF])
                pc += 2
            else:
                emit(pc, [OPCODES[(ln.mne, ABSY)], val & 0xFF, (val >> 8) & 0xFF])
                pc += 3
        else:
            if is_force_zp(expr) and (ln.mne, ZP) in OPCODES:
                emit(pc, [OPCODES[(ln.mne, ZP)], val & 0xFF])
                pc += 2
            else:
                emit(pc, [OPCODES[(ln.mne, ABS)], val & 0xFF, (val >> 8) & 0xFF])
                pc += 3

    if used_hi < used_lo:
        return start, b"", labels
    return used_lo, bytes(image[used_lo : used_hi + 1]), labels


def c_array(name: str, origin: int, data: bytes) -> str:
    hexes = ", ".join(f"0x{b:02X}" for b in data)
    return (
        f"static const uint8_t {name}[] = {{\n    {hexes}\n}};\n"
        f"static const uint16_t {name}_org = 0x{origin:04X};\n"
        f"static const unsigned {name}_len = {len(data)};\n"
    )


def main() -> None:
    if len(sys.argv) < 2:
        print("usage: asm.py file.asm [org]", file=sys.stderr)
        sys.exit(2)
    path = Path(sys.argv[1])
    org = int(sys.argv[2], 0) if len(sys.argv) > 2 else 0x0300
    origin, data, labels = assemble(path.read_text(), org)
    print(f"; org ${origin:04X}  size {len(data)}  labels {labels}")
    print(" ".join(f"{b:02X}" for b in data))


if __name__ == "__main__":
    main()
