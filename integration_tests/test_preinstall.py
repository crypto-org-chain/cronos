from .network import Cronos

def test_preinstall(cronos: Cronos):
    """
    check preinstall functionalities
    """
    w3 = cronos.w3
    create2address = '0x4e59b44847b379578588920ca78fbf26c0b4956c'
    create2code = w3.eth.get_code(create2address)
    assert create2code == 0
