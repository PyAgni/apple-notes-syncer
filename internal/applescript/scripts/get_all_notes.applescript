-- get_all_notes.applescript
-- Extracts all notes from Apple Notes.
-- Output format: fields separated by |||FIELD|||, records separated by |||NOTE|||
-- Fields: id, name, body, creation_date, modification_date, account, folder_path, password_protected, shared

set fieldSep to "|||FIELD|||"
set recSep to "|||NOTE|||"
set output to ""

tell application "Notes"
	repeat with acct in every account
		set acctName to name of acct
		repeat with fldr in every folder of acct
			set fldrName to name of fldr

			-- Build full folder path by traversing parent containers.
			set fullPath to fldrName
			try
				set parentRef to container of fldr
				repeat while class of parentRef is folder
					set fullPath to (name of parentRef) & "/" & fullPath
					set parentRef to container of parentRef
				end repeat
			end try

			repeat with n in every note of fldr
				set noteID to id of n
				set noteName to name of n

				-- Get body; password-protected notes may error.
				set noteBody to ""
				try
					set noteBody to body of n
				end try

				set noteCreated to creation date of n as text
				set noteModified to modification date of n as text
				set noteProtected to password protected of n as text
				set noteShared to shared of n as text

				set output to output & noteID & fieldSep & noteName & fieldSep & noteBody & fieldSep & noteCreated & fieldSep & noteModified & fieldSep & acctName & fieldSep & fullPath & fieldSep & noteProtected & fieldSep & noteShared & recSep
			end repeat
		end repeat
	end repeat
	return output
end tell
