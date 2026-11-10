/*
 * The part of libvim's C API that this binding uses: its declarations, written
 * out here so that the package compiles against this one file instead of a
 * copy of libvim's whole header tree (libvim.h includes vim.h, which includes
 * everything: over a hundred files).
 *
 * These are copies, so they can go out of step with the library they
 * describe, and the compiler that builds vimango cannot tell: it never sees
 * libvim's own headers. TestHeaderMatchesLibvim (header_test.go) can. It
 * compiles this file together with the fork's libvim.h, so a prototype that
 * differs does not compile, and compares the size and layout of every type
 * and the value of every constant. It runs whenever a checkout of the fork is
 * beside this repository, and scripts/build-libvim.sh runs it after every
 * build of libvim.a.
 *
 * To use something more of libvim: copy its declaration from the fork's
 * src/libvim.h below, add any type or constant to the checks in
 * header_test.go, and run the test.
 */
#ifndef VIMANGO_LIBVIM_API_H
#define VIMANGO_LIBVIM_API_H

#include <stddef.h>
#include <stdlib.h>

/*
 * Types. header_test.go defines VIMANGO_CHECK_AGAINST_LIBVIM after including
 * the real libvim.h, to check the prototypes below against the real types.
 */
#ifndef VIMANGO_CHECK_AGAINST_LIBVIM

typedef unsigned char char_u;
typedef long linenr_T; /* line number, 1-based */
typedef int colnr_T;   /* column, a 0-based byte offset */

typedef struct
{
  linenr_T lnum;
  colnr_T col;
  colnr_T coladd;
} pos_T;

typedef struct file_buffer buf_T; /* only ever held by pointer */

typedef enum
{
  MSG_INFO,
  MSG_WARNING,
  MSG_ERROR,
} msgPriority_T;

typedef void (*VoidCallback)(void);
typedef void (*MessageCallback)(char_u *title, char_u *msg, msgPriority_T priority);

typedef struct
{
  pos_T start;
  pos_T end;
} searchHighlight_T;

typedef enum
{
  SPELL_ERROR_BAD,  /* word not recognized */
  SPELL_ERROR_CAPS, /* word should start with a capital */
  SPELL_ERROR_RARE, /* word is rarely used */
  SPELL_ERROR_LOCAL /* word is only used in another region */
} spellErrorType_T;

typedef struct
{
  linenr_T lnum;
  colnr_T col; /* bytes */
  int len;     /* bytes */
  spellErrorType_T type;
} spellError_T;

/* What a function returning int for success returns. */
#define OK 1

/* vimGetMode's values. */
#define NORMAL 0x01
#define VISUAL 0x02
#define OP_PENDING 0x04
#define CMDLINE 0x08
#define INSERT 0x10
#define NORMAL_BUSY (0x100 + NORMAL)

#endif /* VIMANGO_CHECK_AGAINST_LIBVIM */

/* vim's globals. */
extern buf_T *curbuf;  /* the current buffer */
extern int restricted; /* restricted mode, as "vim -Z": no program is started */

void vim_free(void *x);

void vimInit(int argc, char **argv);

buf_T *vimBufferNew(int flags);
void vimBufferSetCurrent(buf_T *buf);
int vimBufferGetId(buf_T *buf);
long vimBufferGetLastChangedTick(buf_T *buf);
char_u *vimBufferGetLine(buf_T *buf, linenr_T lnum);
size_t vimBufferGetLineCount(buf_T *buf);
void vimBufferSetLines(buf_T *buf, linenr_T start, linenr_T end, char_u **lines, int count);

void vimInput(char_u *input);
void vimKey(char_u *key);
void vimExecute(char_u *cmd);
char_u *vimEval(char_u *str);
int vimGetMode(void);

pos_T vimCursorGetPosition(void);
void vimCursorSetPosition(pos_T pos);

int vimVisualGetType(void);
void vimVisualGetRange(pos_T *startPos, pos_T *endPos);

char_u vimCommandLineGetType(void);
char_u *vimCommandLineGetText(void);
int vimCommandLineGetPosition(void);

void vimSetMappingsEnabled(int enabled);
void vimSetFileWritesEnabled(int enabled);
void vimSetMessageCallback(MessageCallback messageCallback);

pos_T *vimSearchGetMatchingPair(int initc);
char_u *vimSearchGetPattern(void);
void vimSearchGetHighlights(buf_T *buf, linenr_T start_lnum, linenr_T end_lnum,
                            int *num_highlights, searchHighlight_T **highlights);
int vimSearchGetHighlightEnabled(void);

void vimSpellGetErrors(buf_T *buf, linenr_T start_lnum, linenr_T end_lnum,
                       int *num_errors, spellError_T **errors);
int vimSpellGetSuggestions(char_u **badWord, int *num_suggestions, char_u ***suggestions);
void vimSetSpellSuggestCallback(VoidCallback callback);

#endif /* VIMANGO_LIBVIM_API_H */
