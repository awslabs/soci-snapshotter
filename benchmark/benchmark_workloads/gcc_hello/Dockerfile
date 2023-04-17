# READYLINE: Hello world

FROM gcc:11.1.0
RUN mkdir temp && printf "#include <stdio.h> \n int main() { \nprintf(\"Hello world\"); return 0; \n }" > temp/hello_world.c
RUN gcc -o hello_world temp/hello_world.c
CMD ["./hello_world"]
