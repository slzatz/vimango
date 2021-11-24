package main

type Organizer struct {
	mode      Mode
	last_mode Mode

	cx, cy    int //cursor x and y position
	fc, fr    int // file x and y position
	rowoff    int //the number of rows scrolled (aka number of top rows now off-screen
	altRowoff int
	coloff    int //the number of columns scrolled (aka number of left rows now off-screen

	rows         []Row
	altRows      []AltRow
	altFr        int
	filter       string
	sort         string
	command_line string
	message      string
	note         []string // the preview

	command string
	repeat  int

	show_deleted   bool
	show_completed bool

	view            View
	altView         int
	taskview        int
	current_task_id int
	string_buffer   string

	context_map map[string]int
	//idToContext map[int]string
	folder_map map[string]int
	//idToFolder  map[int]string
	sort_map   map[string]int
	keywordMap map[string]int

	marked_entries map[int]struct{} // map instead of list makes toggling a row easier

	title_search_string string
	highlight           [2]int

	*Session
}
