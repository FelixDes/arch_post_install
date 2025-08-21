cmake_minimum_required(VERSION 3.31)
project(arch_post_install)

set(CMAKE_CXX_STANDARD 26)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

include(FetchContent)

find_package(Curses REQUIRED)
find_package(CURL REQUIRED)

# ---- cxxopts ----
FetchContent_Declare(
        cxxopts
        GIT_REPOSITORY https://github.com/jarro2783/cxxopts.git
        GIT_TAG v3.2.0
)
FetchContent_MakeAvailable(cxxopts)

# ---- yaml-cpp (static) ----
set(YAML_BUILD_SHARED_LIBS OFF CACHE BOOL "Build yaml-cpp shared libs" FORCE)
set(YAML_CPP_BUILD_TESTS OFF CACHE BOOL "Build yaml-cpp tests" FORCE)

FetchContent_Declare(
        yaml-cpp
        GIT_REPOSITORY https://github.com/jbeder/yaml-cpp.git
        GIT_TAG 0.8.0
)
FetchContent_MakeAvailable(yaml-cpp)

# ---- executable ----
add_executable(arch_post_install main.cpp)

# Force static linking where possible
target_link_libraries(arch_post_install
        PRIVATE
        yaml-cpp
        cxxopts
        CURL::libcurl
        ${CURSES_LIBRARIES}
)

# Put binaries in build/bin
set_target_properties(arch_post_install PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/bin
)
