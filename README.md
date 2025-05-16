# Fileserver for Lan Party 🎮 
Designed for straightforward LAN party sharing, this tool enables you to manage files, games, and media in two ways: either by preloading data to build an image in advance or by uploading content dynamically as the situation requires.

## Workflows

### Main
[![Binary GoLang](https://github.com/R2Unit/lanparty-fileserver/actions/workflows/build-golang-binary.yml/badge.svg?branch=main)](https://github.com/R2Unit/lanparty-fileserver/actions/workflows/build-golang-binary.yml)

### Develop
[![Binary GoLang](https://github.com/R2Unit/lanparty-fileserver/actions/workflows/build-golang-binary.yml/badge.svg?branch=develop)](https://github.com/R2Unit/lanparty-fileserver/actions/workflows/build-golang-binary.yml)

## How to use it.

### Virtual Machine (VM)

- **Step 1:** Install Docker

Make sure you have docker installed. use the official [Docker Install Guide](https://docs.docker.com/engine/install/).

- **Step 2:** Create Direcotry

If you have docker installed do the following in your terminal.
```bash
sudo mkdir /opt/lanparty-fileserver && cd /opt/lanparty-fileserver\
```

- **Step 3:** Create compose

```bash
echo touch compose.yml
```

- **Step 4:** Copy+Paste compose services

For simplicity we will use nano, do the following in your terminal. 

```bash
sudo nano compose.yml
```

Then copy the **compose.yml** into the repository then you can adjust the **MAX_FILE_SIZE** if you need to.

Then when the changes are done do **CONTROL + C**, and **CONTROL + X** and then type **y** and then hit **ENTER**,

- **Step 5:** Compose up

Then do the following command 

**To run as a background service.**
```bash
sudo docker compose up -d
```

**For testing.**
```bash
sudo docker compose up
```

Happy LAN Party! 🎉