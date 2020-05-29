#!/usr/bin/python
import socket
import sys

if len(sys.argv) != 2:
    print("Usage: " + sys.argv[0] + " <file of usernames>")
    sys.exit(0)

attemptedFileName = "Attempted.txt"
attemptedWithErrorFileName = "Attempted_with_Error.txt"
usernameList = open(sys.argv[1]).readlines()
hostList = ['beep']

# for octect in range(2,254):
stillAttemptingUsers = True
while stillAttemptingUsers is True:

    # print "Reading From Previously Attempted List... ",
    try:
        attemptedList = open(attemptedFileName).readlines()
    except:
        print(attemptedFileName + " not found. Creating it... ", )
        attemptedList = []
    attemptedListFile = open(attemptedFileName, "a", 0)

    try:
        attemptedList2 = open(attemptedWithErrorFileName).readlines()
    except:
        attemptedList2 = []

    attemptedList = attemptedList + attemptedList2
    # print str(len(attemptedList)) + " items found. Stripping white space... ",
    for i in range(1, len(attemptedList)):
        attemptedList[i] = attemptedList[i].strip()
    # print "Done. Moving on to scan"

    for host in hostList:
        stillAttemptingUsers = False
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        # host = '10.11.1.' + str(octect)
        print("(Re)Connecting to: " + host)
        try:
            connect = s.connect((host, 25))
            banner = s.recv(1024)
            # print("\tBanner: " + banner)
            # VRFY a users
            # usernameList.reverse()
            for username in usernameList:
                username = username.strip().replace(" ", "")
                if (host + ":" + username) not in attemptedList and username != "":
                    # print "VRFYing username: '" + username + "' on host " + host
                    s.send('VRFY ' + username + '\r\n')
                    result = s.recv(1024)
                    if "User unknown" in result:
                        stillAttemptingUsers = True
                        message = host + ":" + username + "\n"
                        attemptedListFile.write(message)
                    elif "501" in result:
                        print("ERROR with username: " + username)
                        open("Attempted_with_Error.txt", "a+").write(host + ":" + username + "\n")
                    elif "252" in result:
                        print("Found " + username)  # + " on host: " + host)
                        open("FoundUserNames.txt", "a+").write(host + ":" + username + "\n")
                        message = host + ":" + username + "\n"
                        attemptedListFile.write(message)
                    elif "421 4.7.0" in result:
                        # print("too many incorrect attempts")
                        pass
                    else:
                        print("Warning! Something unexpected happened: " + result)
                        open("Attempted_with_Error.txt", "a+").write(host + ":" + username + "\n")

                else:
                    # print "Not testing. Host:Username was in attemptedList: " + host + ":" + username.strip()
                    pass

            # When we've finished testing all the usernames, close the socket
            s.close()
        except socket.error as e:
            if "No route to host" in e:
                print("Unable to connect to: " + host)
            elif "[Errno 104] Connection reset by peer" in str(e):
                pass  # print("Connection Reset. Re-trying")
            else:
                print("socket Error: " + str(e))
