from pman import PodmanRemote
from utils import write_out, write_err

def cli(subparser):
    imagesp = subparser.add_parser("rmi",
                                   help=("delete one or more images"))
    imagesp.add_argument("--force", "-f", action="store_true", help="force delete", dest="force")
    imagesp.add_argument("delete_targets", nargs='*', help="images to delete")
    imagesp.set_defaults(_class=Rmi, func='remove_images')


class Rmi(PodmanRemote):

    def remove_images(self):
        delete_targets = getattr(self.args, "delete_targets")
        if len(delete_targets) < 1:
            raise ValueError("you must supply at least one image id or name to delete")
        force = getattr(self.args, "force")
        for d in delete_targets:
           image = self.client.images.get(d)
           if image["containers"] > 0 and not force:
               write_err("unable to delete {} because it has associated errors. retry with --force".format(d))
               continue
           image.remove(force)
           write_out(image["id"])
