# bboard
File volume analysis

    Usage of bboard:  
    -details string  
      File to store detail data - csv/xls mode  
    -exclude string  
      Directories to exclude  
    -feedback int
      Display file processing (feedback count)
    -filternull
      Filtering 0 valued line
    -history int
      Keep historical data maximum (default 10)
    -no-color
      Disable color output
    -quickrefresh string
      File to store cached data - quicker search/trend mode
    -readonly
      don't get files. Dump json file
    -src string
      Source file specification
    -verbose
      Verbose mode

>  Samples :  
bboard.exe -src \\frparems01.brinks.Fr\production\in\;\\frparems01.brinks.Fr\production\encours\ -quickrefresh new-ems.json -readonly -filternull  
