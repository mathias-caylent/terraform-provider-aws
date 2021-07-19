package aws

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/aws/aws-sdk-go/service/macie2"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAwsDetectiveGraph() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceDetectiveGraphCreate,
		ReadWithoutTimeout:   resourceDetectiveGraphRead,
		UpdateWithoutTimeout: resourceDetectiveGraphUpdate,
		DeleteWithoutTimeout: resourceDetectiveGraphDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"graph_tags": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
			},
		},
	}
}

func resourceDetectiveGraphCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.CreateGraphInput{}

	if kv, ok := d.GetOk("graph_tags"); ok {
		input.Tags = expandStringMap(kv.(map[string]interface{}))
	}

	var err error
	var res *detective.CreateGraphOutput
	err = resource.RetryContext(ctx, 4*time.Minute, func() *resource.RetryError {
		res, err = conn.CreateGraphWithContext(ctx, input)
		if err != nil {
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if isResourceTimeoutError(err) {
		res, err = conn.CreateGraphWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating graph: %w", err))
	}

	d.SetId(*res.GraphArn)

	return resourceDetectiveGraphRead(ctx, d, meta)
}

func resourceDetectiveGraphRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.ListTagsForResourceInput{
		ResourceArn: aws.String(d.Id()),
	}

	resp, err := conn.ListTagsForResourceWithContext(ctx, input)

	if tfawserr.ErrCodeEquals(err, detective.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Graph resource (%s) does not seem to exist, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading Detective Graph (%s): %w", d.Id(), err))
	}

	d.Set("graph_tags", resp.Tags)

	return nil
}

func resourceDetectiveGraphUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	// Retrieve current values
	tagsInput := &detective.ListTagsForResourceInput{
		ResourceArn: aws.String(d.Id()),
	}

	respTags, errTags := conn.ListTagsForResourceWithContext(ctx, tagsInput)

	if tfawserr.ErrCodeEquals(errTags, detective.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Graph resource (%s) does not seem to exist, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if errTags != nil {
		return diag.FromErr(fmt.Errorf("error reading Detective Graph (%s): %w", d.Id(), errTags))
	}

	// Delete current values
	deleteInput := &detective.UntagResourceInput{
		ResourceArn: aws.String(d.Id()),
	}

	tagKeys := []*string{}
	for tagKey := range respTags.Tags {
		tagKeys = append(tagKeys, aws.String(tagKey))
	}

	deleteInput.TagKeys = tagKeys

	_, errUntag := conn.UntagResourceWithContext(ctx, deleteInput)

	if tfawserr.ErrCodeEquals(errUntag, detective.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Graph resource (%s) does not seem to exist, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if errUntag != nil {
		return diag.FromErr(fmt.Errorf("error untagging Detective Graph (%s): %w", d.Id(), errUntag))
	}

	// Tag with new values
	input := &detective.TagResourceInput{
		ResourceArn: aws.String(d.Id()),
	}

	if kv, ok := d.GetOk("graph_tags"); ok && d.HasChange("graph_tags") {
		input.Tags = expandStringMap(kv.(map[string]interface{}))
	}

	_, err := conn.TagResourceWithContext(ctx, input)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error updating Detective graph (%s): %w", d.Id(), err))
	}

	return resourceMacie2AccountRead(ctx, d, meta)
}

func resourceDetectiveGraphDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*AWSClient).detectiveconn

	input := &detective.DeleteGraphInput{
		GraphArn: aws.String(d.Id()),
	}

	err := resource.RetryContext(ctx, 4*time.Minute, func() *resource.RetryError {
		_, err := conn.DeleteGraphWithContext(ctx, input)

		if err != nil {
			if tfawserr.ErrCodeEquals(err, macie2.ErrCodeResourceNotFoundException) ||
				tfawserr.ErrMessageContains(err, macie2.ErrCodeAccessDeniedException, "Macie is not enabled") {
				return nil
			}
			return resource.NonRetryableError(err)
		}

		return nil
	})

	if isResourceTimeoutError(err) {
		_, err = conn.DeleteGraphWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error delete Detective graph (%s): %w", d.Id(), err))
	}

	return nil
}
