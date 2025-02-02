# Scripts

## Auto deploy
- Clone a bare git repo
- Create the `post-receive` in hooks
- Add the git repo as a remote
- After pushing to main, the deployment should happen

## Ansible

- Run ping module to check the inventory: `ansible all -i hosts -m ping`
- Run the playbook: `ansible-playbook -i hosts setup.yml -v`
