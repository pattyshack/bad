.global main # expose main label to global scope

.section .data

hello_world:       .asciz "hello world!\n"
hex_format:        .asciz "%#x"  # local label for printf
float_format:      .asciz "%.2f"
long_float_format: .asciz "%.2Lf"

.section .text

# kill -5 <pid>
.macro trap
  movq $62, %rax # setup syscall id 62 = kill
  movq %r12, %rdi # first arg, pid (r12 has pid)
  mov $5, %rsi  # second arg, signal 5 = SIGTRAP
  syscall
.endm

.macro print_hello_world
  leaq hello_world(%rip), %rdi
  movq $0, %rax
  call printf@plt
  movq $0, %rdi
  call fflush@plt
.endm

.macro print_rsi_hex
  # printf's 1st arg (rdi): hex_format address.  The address is relative to
  # rip due to -pie flag.
  leaq hex_format(%rip), %rdi

  # set number of vector registers involved to 0
  movq $0, %rax

  # @plt - locate shared lib function from procedure linkage table
  call printf@plt

  # fflush's 1st arg (rdi): 0
  movq $0, %rdi

  call fflush@plt
.endm

main:
  push %rbp  # push previous stack pointer to stack
  movq %rsp, %rbp  # save current stack pointer

  # get pid
  movq $39, %rax # setup syscall id 39 = get pid
  syscall

  # %r12 = <pid>
  movq %rax, %r12

  trap

  print_rsi_hex

  trap

  # print contents of mm0
  movq %mm0, %rsi

  print_rsi_hex

  trap

  # print contents of xmm0
  leaq float_format(%rip), %rdi  # 1st arg (rdi): float template
  movq $1, %rax                  # number of vector registers involved
  call printf@plt
  movq $0, %rdi
  call fflush@plt

  trap

  # print contents of st0

  subq $16, %rsp  # push 16 bytes on to the function stack

  fstpt (%rsp)  # pop value from fpu stack to function stack
  leaq long_float_format(%rip), %rdi  # printf's 1st arg (rdi)
  movq $0, %rax # number of vector registers involved
  call printf@plt
  movq $0, %rdi
  call fflush@plt

  addq $16, %rsp # pop 16 bytes from the function stack

  trap

  popq %rbp  # restore previous stack pointer from stack
  movq $0, %rax  # set return value to 0
  ret
