from pystarport.utils import interact


class GoRc:
    def __init__(self, config_path):
        self.config_path = config_path

    def sign_validator(self, eth_key_name, val_addr, nonce):
        return (
            interact(
                f"gorc -c {self.config_path} sign-delegate-keys "
                f"{eth_key_name} {val_addr} {nonce}"
            )
            .strip()
            .decode()
        )

    def add_eth_key(self, name):
        interact(f"gorc -c {self.config_path} keys eth add {name}")

    def add_cosmos_key(self, name):
        interact(f"gorc -c {self.config_path} keys cosmos add {name}")

    def show_eth_addr(self, name):
        return (
            interact(f"gorc -c {self.config_path} keys eth show {name}")
            .strip()
            .decode()
        )

    def show_cosmos_addr(self, name):
        return (
            interact(f"gorc -c {self.config_path} keys cosmos show {name}")
            .split()[1]
            .decode()
        )
