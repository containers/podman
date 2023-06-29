package release

import (
	"github.com/coreos/stream-metadata-go/stream"
	"github.com/coreos/stream-metadata-go/stream/rhcos"
)

func mapArtifact(ra *Artifact) *stream.Artifact {
	if ra == nil {
		return nil
	}
	return &stream.Artifact{
		Location:           ra.Location,
		Signature:          ra.Signature,
		Sha256:             ra.Sha256,
		UncompressedSha256: ra.UncompressedSha256,
	}
}

func mapFormats(m map[string]ImageFormat) map[string]stream.ImageFormat {
	r := make(map[string]stream.ImageFormat)
	for k, v := range m {
		r[k] = stream.ImageFormat{
			Disk:      mapArtifact(v.Disk),
			Kernel:    mapArtifact(v.Kernel),
			Initramfs: mapArtifact(v.Initramfs),
			Rootfs:    mapArtifact(v.Rootfs),
		}
	}
	return r
}

// Convert a release architecture to a stream architecture
func (releaseArch *Arch) toStreamArch(rel *Release) stream.Arch {
	artifacts := make(map[string]stream.PlatformArtifacts)
	cloudImages := stream.Images{}
	var rhcosExt *rhcos.Extensions
	relRHCOSExt := releaseArch.RHELCoreOSExtensions
	if relRHCOSExt != nil {
		rhcosExt = &rhcos.Extensions{}
	}

	if releaseArch.Media.Aliyun != nil {
		artifacts["aliyun"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Aliyun.Artifacts),
		}
		aliyunImages := stream.ReplicatedImage{
			Regions: make(map[string]stream.SingleImage),
		}
		if releaseArch.Media.Aliyun.Images != nil {
			for region, image := range releaseArch.Media.Aliyun.Images {
				si := stream.SingleImage{Release: rel.Release, Image: image.Image}
				aliyunImages.Regions[region] = si

			}
			cloudImages.Aliyun = &aliyunImages
		}
	}

	if releaseArch.Media.Aws != nil {
		artifacts["aws"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Aws.Artifacts),
		}
		awsAmis := stream.ReplicatedImage{
			Regions: make(map[string]stream.SingleImage),
		}
		if releaseArch.Media.Aws.Images != nil {
			for region, ami := range releaseArch.Media.Aws.Images {
				si := stream.SingleImage{Release: rel.Release, Image: ami.Image}
				awsAmis.Regions[region] = si

			}
			cloudImages.Aws = &awsAmis
		}
	}

	if releaseArch.Media.Azure != nil {
		artifacts["azure"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Azure.Artifacts),
		}

		if relRHCOSExt != nil {
			az := relRHCOSExt.AzureDisk
			if az != nil {
				rhcosExt.AzureDisk = &rhcos.AzureDisk{
					Release: rel.Release,
					URL:     az.URL,
				}
			}
		}
		// In the future this is where we'd also add FCOS Marketplace data.
		// See https://github.com/coreos/stream-metadata-go/issues/13
	}

	if releaseArch.Media.AzureStack != nil {
		artifacts["azurestack"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.AzureStack.Artifacts),
		}
	}

	if releaseArch.Media.Digitalocean != nil {
		artifacts["digitalocean"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Digitalocean.Artifacts),
		}

		/* We're producing artifacts but they're not yet available
		   in DigitalOcean as distribution images.
		digitalOceanImage := stream.CloudImage{Image: fmt.Sprintf("fedora-coreos-%s", Stream)}
		cloudImages.Digitalocean = &digitalOceanImage
		*/
	}

	if releaseArch.Media.Exoscale != nil {
		artifacts["exoscale"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Exoscale.Artifacts),
		}
	}

	if releaseArch.Media.Gcp != nil {
		artifacts["gcp"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Gcp.Artifacts),
		}

		if releaseArch.Media.Gcp.Image != nil {
			cloudImages.Gcp = &stream.GcpImage{
				Release: rel.Release,
				Name:    releaseArch.Media.Gcp.Image.Name,
				Family:  releaseArch.Media.Gcp.Image.Family,
				Project: releaseArch.Media.Gcp.Image.Project,
			}
		}
	}

	if releaseArch.Media.HyperV != nil {
		artifacts["hyperv"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.HyperV.Artifacts),
		}
	}

	if releaseArch.Media.Ibmcloud != nil {
		artifacts["ibmcloud"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Ibmcloud.Artifacts),
		}
		ibmcloudObjects := stream.ReplicatedObject{
			Regions: make(map[string]stream.SingleObject),
		}
		if releaseArch.Media.Ibmcloud.Images != nil {
			for region, object := range releaseArch.Media.Ibmcloud.Images {
				so := stream.SingleObject{
					Release: rel.Release,
					Object:  object.Object,
					Bucket:  object.Bucket,
					Url:     object.Url,
				}
				ibmcloudObjects.Regions[region] = so

			}
			cloudImages.Ibmcloud = &ibmcloudObjects
		}
	}

	if releaseArch.Media.KubeVirt != nil {
		artifacts["kubevirt"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.KubeVirt.Artifacts),
		}
		if releaseArch.Media.KubeVirt.Image != nil {
			cloudImages.KubeVirt = &stream.ContainerImage{
				Release:   rel.Release,
				Image:     releaseArch.Media.KubeVirt.Image.Image,
				DigestRef: releaseArch.Media.KubeVirt.Image.DigestRef,
			}
		}
	}

	if releaseArch.Media.Metal != nil {
		artifacts["metal"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Metal.Artifacts),
		}
	}

	if releaseArch.Media.Nutanix != nil {
		artifacts["nutanix"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Nutanix.Artifacts),
		}
	}

	if releaseArch.Media.Openstack != nil {
		artifacts["openstack"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Openstack.Artifacts),
		}
	}

	// if releaseArch.Media.Packet != nil {
	// 	packet := StreamMediaDetails{
	// 		Release: rel.Release,
	// 		Formats: releaseArch.Media.Packet.Artifacts,
	// 	}
	// 	artifacts.Packet = &packet

	// 	packetImage := StreamCloudImage{Image: fmt.Sprintf("fedora_coreos_%s", rel.Stream)}
	// 	cloudImages.Packet = &packetImage
	// }

	if releaseArch.Media.PowerVS != nil {
		artifacts["powervs"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.PowerVS.Artifacts),
		}
		powervsObjects := stream.ReplicatedObject{
			Regions: make(map[string]stream.SingleObject),
		}
		if releaseArch.Media.PowerVS.Images != nil {
			for region, object := range releaseArch.Media.PowerVS.Images {
				so := stream.SingleObject{
					Release: rel.Release,
					Object:  object.Object,
					Bucket:  object.Bucket,
					Url:     object.Url,
				}
				powervsObjects.Regions[region] = so

			}
			cloudImages.PowerVS = &powervsObjects
		}
	}

	if releaseArch.Media.Qemu != nil {
		artifacts["qemu"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Qemu.Artifacts),
		}
	}

	if releaseArch.Media.QemuSecex != nil {
		artifacts["qemu-secex"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.QemuSecex.Artifacts),
		}
	}

	if releaseArch.Media.VirtualBox != nil {
		artifacts["virtualbox"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.VirtualBox.Artifacts),
		}
	}

	if releaseArch.Media.Vmware != nil {
		artifacts["vmware"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Vmware.Artifacts),
		}
	}

	if releaseArch.Media.Vultr != nil {
		artifacts["vultr"] = stream.PlatformArtifacts{
			Release: rel.Release,
			Formats: mapFormats(releaseArch.Media.Vultr.Artifacts),
		}
	}

	return stream.Arch{
		Artifacts:            artifacts,
		Images:               cloudImages,
		RHELCoreOSExtensions: rhcosExt,
	}
}

// ToStreamArchitectures converts a release to a stream
func (rel *Release) ToStreamArchitectures() map[string]stream.Arch {
	streamArch := make(map[string]stream.Arch)
	for arch, releaseArch := range rel.Architectures {
		streamArch[arch] = releaseArch.toStreamArch(rel)
	}
	return streamArch
}
