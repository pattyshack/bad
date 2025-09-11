cmake_minimum_required(VERSION 3.22)

project("targets" LANGUAGES CXX ASM)

function(add_test_cpp_target name)
  add_executable(${name} "${name}.cpp")
  target_compile_options(${name} PRIVATE -g -O0 -pie)
endfunction()

function(add_test_asm_target name)
  add_executable(${name} "${name}.s")
  target_compile_options(${name} PRIVATE -pie)
endfunction()

add_test_cpp_target(hello_world)
add_test_cpp_target(memory)
add_test_cpp_target(print_longdouble)
add_test_cpp_target(run_endlessly)

add_test_asm_target(reg_write)
add_test_asm_target(reg_read)
