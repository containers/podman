from pman import PodmanRemote
from utils import write_out, convert_size, stringTimeToHuman

def cli(subparser):
    imagesp = subparser.add_parser("rm",
                                   help=("delete one or more containers"))
    imagesp.add_argument("--force", "-f", action="store_true", help="force delete", dest="force")
    imagesp.add_argument("delete_targets", nargs='*', help="container images to delete")
    imagesp.set_defaults(_class=Rm, func='remove_containers')


class Rm(PodmanRemote):

    def remove_containers(self):
        delete_targets = getattr(self.args, "delete_targets")
        if len(delete_targets) < 1:
            raise ValueError("you must supply at least one container id or name to delete")
        force = getattr(self.args, "force")
        for d in delete_targets:
            con = self.client.containers.get(d)
            con.remove(force)
            write_out(con["id"])
