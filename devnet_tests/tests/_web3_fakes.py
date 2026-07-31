ADDRESS = "0x" + "1" * 40


class FakeBuildable:
    def __init__(self, base_tx):
        self._base_tx = base_tx

    def build_transaction(self, overrides):
        return {**self._base_tx, **overrides}
