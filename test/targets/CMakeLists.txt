cmake_minimum_required(VERSION 3.22)

project("targets" LANGUAGES CXX ASM)

add_executable(print_longdouble print_longdouble.cpp)

add_executable(run_endlessly run_endlessly.cpp)

add_executable(reg_write reg_write.s)
target_compile_options(reg_write PRIVATE -pie)

add_executable(reg_read reg_read.s)
target_compile_options(reg_read PRIVATE -pie)
