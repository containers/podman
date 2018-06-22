from pman import PodmanRemote
from utils import write_out, convert_size, stringTimeToHuman

def cli(subparser):
    imagesp = subparser.add_parser("ps",
                                   help=("list containers"))
    imagesp.add_argument("all", action="store_true", help="list all containers")
    imagesp.set_defaults(_class=Ps, func='display_all_containers')


class Ps(PodmanRemote):

    def display_all_containers(self):
        col_fmt = "{0:15}{1:32}{2:22}{3:14}{4:12}{5:30}{6:20}"
        write_out(col_fmt.format("CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS", "NAMES"))

        for i in self.client.containers.list():
            command = " ".join(i["command"])
            write_out(col_fmt.format(i["id"][0:12], i["image"][0:30], command[0:20], stringTimeToHuman(i["createdat"]), i["status"], "", i["names"][0:20]))
