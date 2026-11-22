// The general pasteboard, through AppKit. See clipboard_darwin.go.

#import <AppKit/AppKit.h>
#include <stdlib.h>
#include <string.h>

// The pasteboard of that name; the general one for no name.
static NSPasteboard *board(const char *name)
{
  if (name == NULL || *name == 0)
    return [NSPasteboard generalPasteboard];
  return [NSPasteboard pasteboardWithName:[NSString stringWithUTF8String:name]];
}

// The pasteboard's text as UTF-8 from malloc(), or NULL when it has none.
char *vimangoPasteboardText(const char *name)
{
  @autoreleasepool
  {
    NSString *text = [board(name) stringForType:NSPasteboardTypeString];
    const char *utf8 = text == nil ? NULL : [text UTF8String];
    return utf8 == NULL ? NULL : strdup(utf8);
  }
}

// Replaces what the pasteboard holds with text (UTF-8). Returns 1 if it did.
int vimangoPasteboardSetText(const char *name, const char *text)
{
  @autoreleasepool
  {
    NSString *string = [NSString stringWithUTF8String:text];
    NSPasteboard *pasteboard = board(name);
    if (string == nil || pasteboard == nil)
      return 0;
    [pasteboard clearContents];
    return [pasteboard setString:string forType:NSPasteboardTypeString] ? 1 : 0;
  }
}

// Frees a named pasteboard, which otherwise outlives the process.
void vimangoPasteboardRelease(const char *name)
{
  @autoreleasepool
  {
    [board(name) releaseGlobally];
  }
}
