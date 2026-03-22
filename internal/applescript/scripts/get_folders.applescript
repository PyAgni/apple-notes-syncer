-- get_folders.applescript
-- Extracts the folder hierarchy from Apple Notes.
-- Output format: one folder per line, fields separated by |||FIELD|||
-- Fields: account_name, folder_name, folder_id, folder_path

set fieldSep to "|||FIELD|||"
set recSep to "|||FOLDER|||"
set output to ""

tell application "Notes"
	repeat with acct in every account
		set acctName to name of acct
		repeat with fldr in every folder of acct
			set fldrName to name of fldr
			set fldrID to id of fldr

			-- Build full path by traversing parent containers.
			set fullPath to fldrName
			try
				set parentRef to container of fldr
				repeat while class of parentRef is folder
					set fullPath to (name of parentRef) & "/" & fullPath
					set parentRef to container of parentRef
				end repeat
			end try

			set output to output & acctName & fieldSep & fldrName & fieldSep & fldrID & fieldSep & fullPath & recSep
		end repeat
	end repeat
	return output
end tell
