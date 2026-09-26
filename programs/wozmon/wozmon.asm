;;; Soft-adapted WOZ Monitor for mos6502-logos tools/asm.py
; Soft≠Physical. Classic Apple-1 WOZMON (~256 bytes @ $FF00).
; See platforms/apple1/SOURCE.txt for URL / license / SHA.
; ZP operands use $nn so asm.py selects zp mode (labels would force abs).
; Binary must match platforms/apple1/wozmon.bin.

IN    = $0200
KBD   = $D010
KBDCR = $D011
DSP   = $D012
DSPCR = $D013

        .org $FF00

RESET:  CLD
        CLI
        LDY #$7F
        STY DSP
        LDA #$A7
        STA KBDCR
        STA DSPCR
NOTCR:  CMP #$DF
        BEQ BACKSPACE
        CMP #$9B
        BEQ ESCAPE
        INY
        BPL NEXTCHAR
ESCAPE: LDA #$DC
        JSR ECHO
GETLINE: LDA #$8D
        JSR ECHO
        LDY #$01
BACKSPACE: DEY
        BMI GETLINE
NEXTCHAR: LDA KBDCR
        BPL NEXTCHAR
        LDA KBD
        STA IN,Y
        JSR ECHO
        CMP #$8D
        BNE NOTCR
        LDY #$FF
        LDA #$00
        TAX
SETSTOR: ASL
SETMODE: STA $2B
BLSKIP: INY
NEXTITEM: LDA IN,Y
        CMP #$8D
        BEQ GETLINE
        CMP #$AE
        BCC BLSKIP
        BEQ SETMODE
        CMP #$BA
        BEQ SETSTOR
        CMP #$D2
        BEQ RUN
        STX $28
        STX $29
        STY $2A
NEXTHEX: LDA IN,Y
        EOR #$B0
        CMP #$0A
        BCC DIG
        ADC #$88
        CMP #$FA
        BCC NOTHEX
DIG:    ASL
        ASL
        ASL
        ASL
        LDX #$04
HEXSHIFT: ASL
        ROL $28
        ROL $29
        DEX
        BNE HEXSHIFT
        INY
        BNE NEXTHEX
NOTHEX: CPY $2A
        BEQ ESCAPE
        BIT $2B
        BVC NOTSTOR
        LDA $28
        STA ($26,X)
        INC $26
        BNE NEXTITEM
        INC $27
TONEXTITEM: JMP NEXTITEM
RUN:    JMP ($24)
NOTSTOR: BMI XAMNEXT
        LDX #$02
SETADR: LDA $27,X
        STA $25,X
        STA $23,X
        DEX
        BNE SETADR
NXTPRNT: BNE PRDATA
        LDA #$8D
        JSR ECHO
        LDA $25
        JSR PRBYTE
        LDA $24
        JSR PRBYTE
        LDA #$BA
        JSR ECHO
PRDATA: LDA #$A0
        JSR ECHO
        LDA ($24,X)
        JSR PRBYTE
XAMNEXT: STX $2B
        LDA $24
        CMP $28
        LDA $25
        SBC $29
        BCS TONEXTITEM
        INC $24
        BNE MOD8CHK
        INC $25
MOD8CHK: LDA $24
        AND #$07
        BPL NXTPRNT
PRBYTE: PHA
        LSR
        LSR
        LSR
        LSR
        JSR PRHEX
        PLA
PRHEX:  AND #$0F
        ORA #$B0
        CMP #$BA
        BCC ECHO
        ADC #$06
ECHO:   BIT DSP
        BMI ECHO
        STA DSP
        RTS
        BRK
        BRK
        .WORD $0F00
        .WORD RESET
        .WORD $0000
