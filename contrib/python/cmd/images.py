from pman import PodmanRemote
from utils import write_out, convert_size, stringTimeToHuman

def cli(subparser):
    imagesp = subparser.add_parser("images",
                                   help=("list images"))
    imagesp.add_argument("all", action="store_true", help="list all images")
    imagesp.set_defaults(_class=Images, func='display_all_image_info')


class Images(PodmanRemote):

    def display_all_image_info(self):
        col_fmt = "{0:40}{1:12}{2:14}{3:18}{4:14}"
        write_out(col_fmt.format("REPOSITORY", "TAG", "IMAGE ID", "CREATED", "SIZE"))
        for i in self.client.images.list():
            for r in i["repoTags"]:
                rsplit = r.rindex(":")
                name = r[0:rsplit-1]
                tag = r[rsplit+1:]
                write_out(col_fmt.format(name, tag, i["id"][:12], stringTimeToHuman(i["created"]), convert_size(i["size"])))
