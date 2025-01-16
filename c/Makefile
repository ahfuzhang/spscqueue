
all: output/spsc_queue.o output/test_main
	@echo "ok"

LIBS=""
# 检测操作系统并设置变量
ifeq ($(shell uname), Linux)
    LIBS="-lrt"
endif

output/spsc_queue.o: src/spsc_queue.c include/spsc_queue.h
	gcc -c src/spsc_queue.c -o output/spsc_queue.o -I"include" -fPIC -g -Wall

output/test_main.o: ./tests/main.c src/spsc_queue.c include/spsc_queue.h
	gcc -c ./tests/main.c -o output/test_main.o -I"include" -fPIC -g -Wall

output/test_main: output/test_main.o output/spsc_queue.o
	gcc  output/test_main.o output/spsc_queue.o -o output/test_main $(LIBS)

shm_unlink:
	output/test_main shm_unlink

one_by_one:
	output/test_main one_by_one

darwin_arm64_release:
	gcc -o output/spsc_queue_Darwin_arm64.o -c ./src/spsc_queue.c -g -Wall -fPIC -I"./include/" -O2
	gcc -shared -o output/spsc_queue_Darwin_arm64.so output/spsc_queue_Darwin_arm64.o

linux_amd64_release:
	gcc -o output/spsc_queue_Linux_x86_64.o -c ./src/spsc_queue.c -g -Wall -fPIC -I"./include/" -O2
	gcc -shared -o output/spsc_queue_Linux_x86_64.so output/spsc_queue_Linux_x86_64.o
