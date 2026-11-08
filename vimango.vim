" vimango.vim -- vimango's vimrc, sourced into libvim at startup.
"
" Mappings here apply to keys typed in the note editor only. libvim limits:
" single-key left-hand sides only (a two-key lhs like "jk" swallows keys), and
" no <C-r> inside an insert-mode right-hand side. See vimrc.go.

" Visual mode: turn the selected lines into a markdown list. Indentation is
" kept, blank lines are skipped, and the last search pattern is left alone.
xnoremap - :<C-u>keeppatterns '<,'>s/^\s*\zs\ze\S/- /e<CR>
