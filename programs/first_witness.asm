; FIRST WITNESS hand program — Soft≠Physical teaching ROM.
; Assembled by tools/asm.py or embedded as bytes in tb/witness_first_test.go.
; Covers: LDA/STA imm+zp, ADC+C, CMP/BEQ, JSR/RTS, BRK vector.

        .org $8000
start:  lda #$10
        sta $00
        lda #$20
        clc
        adc $00         ; A = $30
        sta $01
        cmp #$30
        beq ok
        lda #$ee        ; fail path (should not run)
ok:     jsr sub
        lda #$55
        brk
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        ; $8020
        .org $8020
sub:    lda #$aa
        sta $02
        rts

        .org $fffc
        .word start
        .org $fffe
        .word $9000
