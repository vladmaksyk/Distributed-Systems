for /l %%x in (1, 1, 5) do (
	go build
   start powershell -NoExit -Command .\process
)