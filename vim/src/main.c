#include <stdio.h>
#include "libvim.h"

void printDone() {
        printf("Done\n");
}

int main(int argc, char** argv) {
  vimInit(argc, argv);
  (void)vimBufferOpen("testfile.txt", 1, 0);
  vimExecute("e!");
  vimKey("<esc>");
  vimKey("<esc>");
  vimInput("g");
  vimInput("g");
  int x = vimCursorGetLine();
  printf("x = %d\n", x);
  vimInput("G");
  x = vimCursorGetLine();
  printf("x = %d\n", x);
  vimInput("g");
  vimInput("g");
  vimInput("0");
  printDone();
  vimInput("I");
  vimInput("a");
  vimInput("b");
  vimInput("c");
  vimKey("<esc>");
  char_u *line = vimBufferGetLine(curbuf, vimCursorGetLine());
  printf("%s\n", line);
  vimInput("I");
  vimInput("x");
  vimInput("y");
  vimInput("z");
  vimKey("<CR>");
  vimKey("<esc>");
  line = vimBufferGetLine(curbuf, vimCursorGetLine()-1);
  printf("%s\n", line);
  vimInput("O");
  vimInput("1");
  vimInput("2");
  vimInput("3");
  vimKey("<esc>");
  x = vimCursorGetLine();
  line = vimBufferGetLine(curbuf, x);
  printf("%s\n", line);
  vimInput("o");
  vimInput("7");
  vimInput("8");
  vimInput("9");
  vimKey("<esc>");
  x = vimCursorGetLine();
  line = vimBufferGetLine(curbuf, x);
  printf("%s\n", line);
  vimInput("A");
  vimInput("f");
  vimInput("g");
  vimInput("h");
  vimKey("<esc>");
  x = vimCursorGetLine();
  line = vimBufferGetLine(curbuf, x);
  printf("%s\n", line);
}
