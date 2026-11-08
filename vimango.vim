" vimango.vim -- vimango's vimrc, sourced into libvim at startup.
"
" Mappings here apply to keys typed in the note editor only. libvim limits:
" single-key left-hand sides only (a two-key lhs like "jk" swallows keys), and
" no <C-r> inside an insert-mode right-hand side. See vimrc.go.

" Toggle markdown bullets on lines first..last. If every non-blank line already
" starts with "- " (after its indentation), the bullets come off; otherwise
" each non-blank line without one gets one. Blank lines are left alone and
" indentation is kept. One setline call, so one u undoes it.
function! VimangoToggleBullets(first, last) abort
  let lines = getline(a:first, a:last)
  let all_bulleted = 1
  let any_content = 0
  for l in lines
    if l =~ '\S'
      let any_content = 1
      if l !~ '^\s*- '
        let all_bulleted = 0
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
    elseif all_bulleted
      call add(out, substitute(l, '^\(\s*\)- ', '\1', ''))
    elseif l =~ '^\s*- '
      call add(out, l)
    else
      call add(out, substitute(l, '^\s*', '&- ', ''))
    endif
  endfor
  if out !=# lines
    call setline(a:first, out)
  endif
endfunction

" Visual mode: toggle the selected lines as a markdown list.
xnoremap - :<C-u>call VimangoToggleBullets(line("'<"), line("'>"))<CR>
