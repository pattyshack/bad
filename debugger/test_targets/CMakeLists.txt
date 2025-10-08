cmake_minimum_required(VERSION 3.22)

project("targets" LANGUAGES CXX ASM)

function(add_test_cpp_target name)
  add_executable(${name} "${name}.cpp")

  # NOTE: gcc's dwarf5 implementation encodes attribute values using formats
  # that are not part of the dwarf5 spec.  Downgrade to dwarf4 for sanity ...
  target_compile_options(${name} PRIVATE -g -O0 -pie -gdwarf-4)
endfunction()

function(add_test_asm_target name)
  add_executable(${name} "${name}.s")
  target_compile_options(${name} PRIVATE -pie)
endfunction()

add_test_cpp_target(anti_debugger)
add_test_cpp_target(global_variable)
add_test_cpp_target(hello_world)
add_test_cpp_target(memory)
add_test_cpp_target(multi_threaded)
add_test_cpp_target(multi_threaded2)
add_test_cpp_target(overloaded)
add_test_cpp_target(print_longdouble)
add_test_cpp_target(run_endlessly)
add_test_cpp_target(step)

add_test_cpp_target(marshmallow)
add_library(meow SHARED "libmeow.cpp")
target_compile_options(meow PRIVATE -g -O0 -fPIC -gdwarf-4)
target_link_libraries(marshmallow PRIVATE meow)

add_executable(multi_cu multi_cu_main.cpp multi_cu_other.cpp)
target_compile_options(multi_cu PRIVATE -g -O0 -pie -gdwarf-4)

add_test_asm_target(reg_write)
add_test_asm_target(reg_read)
