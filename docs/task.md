Exemple task.yaml file in /etc/serversupervisor
```
tasks:
  - id: docker-pull-teslamate
    name: "docker pull and start Teslamate"
    command: ["bash", "-c", "cd /directory/docker/teslamate && docker compose pull && docker compose up -d"]
    timeout: 3600
  - id: docker-pull-bar-assistant
    name: "docker pull and start BarAssistant"
    command: ["bash", "-c", "cd /directory/docker/bar-assistant && docker compose pull && docker compose up -d"]
    timeout: 3600
  - id: docker-pull-mealie
    name: "docker pull and start Mealie"
    command: ["bash", "-c", "cd /directory/docker/mealie && docker compose pull && docker compose up -d"]
    timeout: 3600
    ```
