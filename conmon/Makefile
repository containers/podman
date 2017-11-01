src = $(wildcard *.c)
obj = $(src:.c=.o)

override LIBS += $(shell pkg-config --libs glib-2.0)
override CFLAGS += -std=c99 -Os -Wall -Wextra $(shell pkg-config --cflags glib-2.0)

conmon: $(obj)
	$(CC) -o ../bin/$@ $^ $(CFLAGS) $(LIBS)

.PHONY: clean
clean:
	rm -f $(obj) ../bin/conmon
