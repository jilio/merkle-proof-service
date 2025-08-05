cat > /etc/sudoers.d/merkle << SUDO
deploy ALL= NOPASSWD: /bin/systemctl restart gala-merkle.service
deploy ALL= NOPASSWD: /bin/systemctl stop gala-merkle.service
deploy ALL= NOPASSWD: /bin/systemctl start gala-merkle.service
SUDO