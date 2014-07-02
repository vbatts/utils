
if exists("loaded_next_note")
    finish
endif
let loaded_next_note = 1

function! s:AppendDatetime()
    let lines = [system("next-note -d")]
    call append( line('$'), lines )
endfunction

function! s:OpenPrevWeeksNote()
    let cmd_output = system("next-note -p")
    if cmd_output == ""
        echohl WarningMsg | 
        \ echomsg "Warning: next note already exists" | 
        \ echohl None
        return
    endif
    execute "tabe " . cmd_output
    execute "tabm +1"
endfunction

function! s:OpenCurrentWeeksNote()
    let cmd_output = system("next-note -c")
    if cmd_output == ""
        echohl WarningMsg | 
        \ echomsg "Warning: next note already exists" | 
        \ echohl None
        return
    endif
    execute "tabe " . cmd_output
    execute "tabm 1"
endfunction

function! s:OpenNextWeeksNote()
    let cmd_output = system("next-note")
    if cmd_output == ""
        echohl WarningMsg | 
        \ echomsg "Warning: next note already exists" | 
        \ echohl None
        return
    endif
    execute "tabe " . cmd_output
    execute "tabm 1"
endfunction

command! Ndate
            \ call s:AppendDatetime()

command! Nprev
            \ call s:OpenPrevWeeksNote()

command! Nnext
            \ call s:OpenNextWeeksNote()

command! Ncurr
            \ call s:OpenCurrentWeeksNote()

