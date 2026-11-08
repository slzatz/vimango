" vimango.vim -- vimango's vimrc, sourced into libvim at startup.
"
" Mappings here apply to keys typed in the note editor only. libvim limits:
" single-key left-hand sides only (a two-key lhs like "jk" swallows keys), and
" no <C-r> inside an insert-mode right-hand side. See vimrc.go.

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
