" vimango.vim -- vimango's vimrc, sourced into libvim at startup.
"
" Mappings here apply to keys typed in the note editor only. A left-hand side
" can be several keys ("inoremap jk <Esc>"); the first waits 'timeoutlen' for
" the rest, as in vim. One libvim limit: no <C-r> inside an insert-mode
" right-hand side. See vimrc.go.

" Markdown list markers, by kind: what is recognized after a line's
" indentation, and what is written. Every numbered line gets "1. " -- markdown
" renders 1, 2, 3, and reordering lines never needs renumbering. "*" only counts
" as a bullet with a space after it, so *italic* text is not mistaken for one.
let s:list_pattern = {'bullet': '[-*+] ', 'number': '\d\+[.)] '}
let s:list_marker = {'bullet': '- ', 'number': '1. '}

" Toggle lines first..last as a list of kind ('bullet' or 'number'):
"   - every non-blank line already that kind: the markers come off
"   - otherwise each non-blank line becomes that kind: a line of the other kind
"     has its marker swapped, a plain line gets one, a line already that kind
"     is left as it is
" Blank lines are left alone and indentation is kept. One setline call, so one
" u undoes it.
function! VimangoToggleList(first, last, kind) abort
  let other = a:kind ==# 'bullet' ? 'number' : 'bullet'
  let mine = '^\s*\zs' . s:list_pattern[a:kind]
  let theirs = '^\s*\zs' . s:list_pattern[other]
  let marker = s:list_marker[a:kind]

  let lines = getline(a:first, a:last)
  let all_mine = 1
  let any_content = 0
  for l in lines
    if l =~ '\S'
      let any_content = 1
      if l !~ mine
        let all_mine = 0
      endif
    endif
  endfor
  if !any_content
    return
  endif

  let out = []
  for l in lines
    if l !~ '\S'
      call add(out, l)
    elseif all_mine
      call add(out, substitute(l, mine, '', ''))
    elseif l =~ mine
      call add(out, l)
    elseif l =~ theirs
      call add(out, substitute(l, theirs, marker, ''))
    else
      call add(out, substitute(l, '^\s*\zs', marker, ''))
    endif
  endfor
  if out !=# lines
    call setline(a:first, out)
  endif
endfunction

" Visual mode: toggle the selected lines as a bulleted (-) or numbered (#) list.
xnoremap - :<C-u>call VimangoToggleList(line("'<"), line("'>"), 'bullet')<CR>
xnoremap # :<C-u>call VimangoToggleList(line("'<"), line("'>"), 'number')<CR>

" ---------------------------------------------------------------------------
" Bold, italic, code.
"
" Ctrl-B (bold **), Ctrl-I (italic *), Ctrl-E (code `): in normal mode they
" act on the word under the cursor, in visual mode on the selection (one line).
" Pressing the same key again takes the decoration off; a different key
" replaces it (Ctrl-I on **word** gives *word*). Ctrl-I and Tab are the same
" byte in a terminal, so Tab does italic too in normal and visual mode -- not in
" insert mode, which has no mapping here.
"
" This is plain string editing on the line -- never iw or gv. After a
" decoration the text has moved, so vim's word motions and the '< '> marks no
" longer describe it; computing the span here is what makes toggling back off
" work.

" A word for decoration runs up to whitespace or sentence punctuation. * and `
" are deliberately not breaks, so with the cursor anywhere on **word** the word
" is the whole **word** -- markers included -- and the key toggles it off.
let s:word_break = '[[:space:].,;:!?()\[\]{}"<>/\\|]'
let s:is_marker = '[*`]'

" The marker t is wrapped in, if any. ** is tried before *, which is its prefix.
function! s:WrappedIn(t) abort
  for m in ['**', '*', '`']
    let n = len(m)
    if len(a:t) > 2 * n && strpart(a:t, 0, n) ==# m && strpart(a:t, len(a:t) - n) ==# m
      return m
    endif
  endfor
  return ''
endfunction

" Toggle marker on bytes [b, e) of line lnum, then put the cursor on the first
" character of the text, so pressing the key again finds it.
function! s:DecorateSpan(lnum, b, e, marker) abort
  let s = getline(a:lnum)
  let [b, e] = [a:b, a:e]
  " widen across markers just outside the span, in matched pairs, so a word
  " selected or found without its decoration still toggles
  while b > 0 && e < len(s) && s[b - 1] =~ s:is_marker && s[e] =~ s:is_marker
    let b -= 1
    let e += 1
  endwhile

  let t = strpart(s, b, e - b)
  let had = s:WrappedIn(t)
  let inner = substitute(t, '^[*`]*\|[*`]*$', '', 'g')
  if inner ==# ''
    return
  endif
  let new = had ==# a:marker ? inner : a:marker . inner . a:marker
  call setline(a:lnum, strpart(s, 0, b) . new . strpart(s, e))
  call cursor(a:lnum, b + 1 + (had ==# a:marker ? 0 : len(a:marker)))
endfunction

" mode: 'n' for the word under the cursor, or visualmode() for the selection.
function! VimangoDecorate(marker, mode) abort
  if a:mode ==# 'n'
    let lnum = line('.')
    let s = getline(lnum)
    let c = col('.') - 1
    if c >= len(s) || s[c] =~ s:word_break
      return
    endif
    let b = c
    while b > 0 && s[b - 1] !~ s:word_break
      let b -= 1
    endwhile
    let e = c + 1
    while e < len(s) && s[e] !~ s:word_break
      let e += 1
    endwhile
    call s:DecorateSpan(lnum, b, e, a:marker)
    return
  endif

  let lnum = line("'<")
  if line("'>") != lnum
    return
  endif
  let s = getline(lnum)
  if a:mode ==# 'V'
    " the whole line, less its indentation and trailing blanks
    let b = match(s, '\S')
    let e = len(substitute(s, '\s*$', '', ''))
  else
    let b = col("'<") - 1
    " '> is the first byte of the last character; take all of its bytes
    let e = col("'>") - 1 + len(matchstr(strpart(s, col("'>") - 1), '^.'))
    let e = min([e, len(s)])
  endif
  if b < 0 || e <= b
    return
  endif
  call s:DecorateSpan(lnum, b, e, a:marker)
endfunction

nnoremap <C-b> :<C-u>call VimangoDecorate('**', 'n')<CR>
nnoremap <C-i> :<C-u>call VimangoDecorate('*', 'n')<CR>
nnoremap <C-e> :<C-u>call VimangoDecorate('`', 'n')<CR>
xnoremap <C-b> :<C-u>call VimangoDecorate('**', visualmode())<CR>
xnoremap <C-i> :<C-u>call VimangoDecorate('*', visualmode())<CR>
xnoremap <C-e> :<C-u>call VimangoDecorate('`', visualmode())<CR>

" ---------------------------------------------------------------------------
" Spell checking (:spell toggles it; see spellcheck.go).
"
" No capital check: vim otherwise flags a lowercase first word after a blank
" line or a sentence end, which notes are full of. Misspellings are still
" highlighted.
set spellcapcheck=

" Unknown words written with capitals are names, not misspellings: not
" highlighted, and ]s [s skip them.
"   noallcaps    capitals only: NASA, MP3
"   nomixedcase  a capital after the first letter: iOS, GitHub, NASA's, PDFs
" A word capitalized only at its start (Smith) is still checked; zg adds it.
" Both values are the libvim fork's own; :syntax and @NoSpell are not available
" (libvim has no syntax engine).
set spelloptions=noallcaps,nomixedcase
