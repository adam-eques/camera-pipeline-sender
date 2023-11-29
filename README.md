 <img align="left" src="https://visitor-badge.laobi.icu/badge?page_id=acentior.camera-pipeline-sender" />
 
# camera-pipeline-sender
Video stream sender connected with camera
 
## Development environment
- Go version: 1.21.4
 
## Work on Ubuntu
### Install Go latest version
You can follow this instruction to install GO latest version on Ubuntu 22.04
 
https://utho.com/docs/tutorial/how-to-install-go-on-ubuntu-22-04/
 
### Install Git latest version
You have to install git before clone this repository.
You can follow this if you didn't installed git on your Ubuntu
 
https://www.digitalocean.com/community/tutorials/how-to-install-git-on-ubuntu-22-04
 
### Development
- You have to clone this repository first
 
```
git clone https://github.com/acentior/camera-pipeline-sender.git
```
- Download needed packages (in the project directory)
```
go mode tidy
```
- You have to create a .env file in the project directory. Please copy and paste this variables.
```
WEBSOCKET_URL=ws://192.168.148.91:8080
```
   Values are example. You can replace url with the signaling server's url on your local network following this format

   ws://ip:port
- Run without a binary file
```
make run
```
- Build a binary file
```
make build
```
- Test
```
make test
```
